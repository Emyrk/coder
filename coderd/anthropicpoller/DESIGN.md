# Research: Anthropic Self-Hosted Sandboxes in Coder

## TL;DR

- A self-hosted sandbox is **not "Anthropic's agent running on your box"**. Claude (the model) and the agentic loop stay in Anthropic's control plane. You run an **environment worker** that polls Anthropic for work items, executes tool calls (bash, file ops) locally, and posts results back.
- Each Anthropic **environment** is a named work queue scoped to one workspace in Anthropic's console. Workers authenticate to one queue with an environment-scoped bearer token (`ANTHROPIC_ENVIRONMENT_KEY`), not the org API key. A "session" is an instance of an agent assigned to an environment; the work item is the per-session execution context.
- The `ant` CLI is a thin Go/MIT wrapper over the official Go SDK. All polling, claim/ack/heartbeat/stop, skill download, and tool execution live in `github.com/anthropics/anthropic-sdk-go/lib/environments`. If we want in-process polling in coderd, we import the **SDK**, not the CLI.
- The PoC at `coder/coder-anthropic-integration-poc` (private repo, 1 commit, never updated since 2026-06-03) wires `ant beta:worker poll --on-work ./start.sh` to a bash script that calls `coder create` per work item, polls for a `/tmp/anthropic-session.done` file over `coder ssh`, then runs `coder delete --yes`. It is the documented per-session-sandbox spawn pattern, with Coder workspaces playing the role of the sandbox container.
- The user's hypothesis is **directionally right but materially off** on several points: what "the agent" is, what the CLI's role is (CLI vs SDK), and how trivial the in-process embedding would be (auth identity, multi-environment fan-out, per-org keys, blast radius). Section 4 has the per-claim breakdown.
- **Top recommendation**: do not pull `ant`/SDK into coderd as a first step. Instead, harden the PoC pattern as a first-class Coder concept ("sandbox poller daemon", peered with `provisionerd`) and design the integration around the `coderd/aitasks` surface that already exists. Move polling in-process only after the per-org identity and key-storage story is settled.

## 1. How Anthropic self-hosted sandboxes work

### What problem they solve

Managed Agents lets Anthropic host an agent loop and serve tool execution out of either Anthropic cloud sandboxes (default) or **your** infrastructure (self-hosted). The docs frame this as: "Self-hosted sandboxes keep the orchestration on Anthropic's side but move tool execution into infrastructure you control". The agent's filesystem, processes, and network egress stay in your environment; tool inputs and outputs still cross to Anthropic so the model can act on them.

Use cases per the docs: data that cannot leave a network boundary, reaching private services, or running under your compliance controls.

### Control plane vs sandbox responsibilities

Anthropic owns:

- The model loop, the prompt, the tool selection, the planner.
- The work queue per environment and the keep-alive/lease semantics.
- The session resource (events, history, status, stop reasons).
- The skill download artifacts and the agent definition.

You own:

- A worker process that polls Anthropic, claims work items, downloads skills, runs the tools, and posts results.
- The sandbox image, network policy, secret storage, log retention.
- Per-environment-key isolation across trust boundaries.

The docs explicitly enumerate what Anthropic cannot do for you: "Instantly invalidate a leaked key", "Verify your worker build", "Isolate tools inside your sandbox", "Enforce data retention in your environment".

### Job and task lifecycle

The unit is a **work item** (a session execution context), not a "job request for compute":

1. **Pre-existing fleet.** You stand up an environment worker (or fleet of workers) ahead of time. Each worker holds an environment key and long-polls one environment's work queue.
2. **Operator or user creates a session.** `POST /v1/sessions` with `agent`, `environment_id`, optional `metadata`. The session is "queued" against the environment.
3. **Worker claims a work item.** Long-poll up to 999 ms server-side, returns a `BetaSelfHostedWork` carrying a `Data.ID` (session ID), `EnvironmentID`, and `WorkID`. Worker posts an `Ack` to confirm the claim. If `Ack` fails, the SDK force-stops the item and discards.
4. **Worker drives the session.** Downloads skills into `<workdir>/skills/<name>/`. Calls `client.beta.sessions.events.tool_runner()` to consume the SSE event stream (`agent.tool_use` and friends), executes each tool, sends results as `user.tool_result`. Heartbeats the work lease while running.
5. **Worker stops the work item.** On session end (`session.status_idle` with `end_turn`) or context cancel, posts `Stop`. Force-stop is available.
6. **Operator polls or webhooks.** Operator can read `work.stats` (depth, pending, oldest_queued_at, workers_polling) with the org API key, and call `work.stop` to graceful-stop a session externally.

Key direction: **the worker always dials out to Anthropic**, both for poll-style work claiming and for the per-session event stream and tool-result posting. The webhook mode is a separate, additive trigger; even with webhooks, the actual work delivery is the worker polling once nudged.

### Authentication model

Two credentials, intentionally separated:

- **`ANTHROPIC_ENVIRONMENT_KEY`** (looks like `sk-ant-oat01-...`). Scoped to a single environment's work queue. The worker holds this. The docs warn: "Store it in a secrets manager, not in environment files or sandbox images". The SDK uses it as an OAuth-style bearer (`Authorization: Bearer ...`) and clears `X-Api-Key` from outgoing requests, see `bearerReqOpts` in `poller.go`.
- **`ANTHROPIC_API_KEY`** (the org-level Console key). Used out-of-band by the operator to create environments, generate environment keys, create sessions, and read stats. The docs include a `<Warning>`: setting this on the worker host "exposes an organization-scoped credential to agent tool calls".

Environment keys are generated only via the Anthropic Console UI, even when the environment itself is created via API. There is no key rotation API documented; rotation is manual.

On AWS Bedrock the worker authenticates with AWS IAM (SigV4) via the `AnthropicSelfHostedEnvironmentAccess` managed policy instead of an environment key.

### Network model

- Worker dials out over HTTPS to `api.anthropic.com` (configurable via `ANTHROPIC_BASE_URL`).
- Long-poll requests stay open up to 999 ms; the SDK loops automatically.
- Webhook mode requires an inbound endpoint on your side that Anthropic can reach, but the actual work delivery still happens via the same long-poll path. The webhook only triggers the worker to start polling.
- Tools that need to reach internal MCP servers can do so directly, since the worker runs in your network. MCP tunnels are an orthogonal feature for when Claude needs to reach a private MCP from Anthropic's cloud sandbox.

### Deployment requirements

The doc lists the worker host requirements concisely: "A Linux host with `/bin/bash` at that exact path". TypeScript SDK additionally needs `unzip`, `tar`, Node 22+. Python SDK has no extra deps. The Go SDK was the example used throughout.

Two operating modes:

- **In-process tools.** `ant beta:worker poll` with no `--on-work`. Tools run as subprocesses of the worker, filesystem isolation is whatever the host gives you. Cheap, low-latency, but a compromised tool can read anything the worker can.
- **Sandbox per session.** `ant beta:worker poll --on-work spawn.sh`. The poller execs `spawn.sh` per work item with the four env vars; the script typically `docker run`s a fresh container that runs `ant beta:worker run` as ENTRYPOINT. Fresh filesystem per session, per-session network policy, output collection via volume mount on `/mnt/session/outputs`.

`workspace` is the default working directory the docs assume; CLI default is `.`. Skills are downloaded to `<workdir>/skills/<name>/`. The worker's `bash` tool is the only one that ignores the `--workdir` containment.

## 2. The `ant` CLI (anthropic-cli)

### What it is

- Repo: https://github.com/anthropics/anthropic-cli
- Language: Go (`go 1.25` in go.mod).
- License: MIT (`Copyright 2023 Anthropic, PBC.`).
- Stainless-generated from an OpenAPI spec, plus hand-written commands. Released by `stainless-app[bot]`. Cadence: roughly weekly minor, near-daily patch. Current version v1.12.2 (2026-06-24).
- Self-hosted sandbox support landed in v1.9.0 (2026-05-19); the changelog entry says: "Add support for self-hosted sandboxes in CMA with sandbox helpers".
- Built around `urfave/cli/v3` and `charmbracelet/bubbletea`. The TUI is for the interactive JSON explorer, not the worker.

### Relevant subcommands

The relevant subcommands for self-hosted sandboxes:

| Command | Source | What it does |
|---|---|---|
| `ant beta:worker poll` | `pkg/cmd/worker.go` | Long-polls an environment; runs the SDK's `EnvironmentWorker.Run` in-process, or execs `--on-work` script per claim |
| `ant beta:worker run` | `pkg/cmd/worker.go` | One-shot per-session sandbox entrypoint; calls SDK's `EnvironmentWorker.HandleItem`; intended as a container `ENTRYPOINT` |
| `ant beta:environments create` | `pkg/cmd/betaenvironment.go` | Create environments (self-hosted, cloud) |
| `ant beta:environments:work stats` | `pkg/cmd/betaenvironmentwork.go` | Operator-facing queue depth read |
| `ant beta:environments:work stop` | `pkg/cmd/betaenvironmentwork.go` | Operator-facing graceful or force stop of a work item |
| `ant beta:environments:work poll/ack/heartbeat/update/list/retrieve` | `pkg/cmd/betaenvironmentwork.go` | Raw API commands, marked "called automatically by the pre-built environment worker... included here as a reference; you do not need to invoke them directly" |
| `ant beta:sessions create` | `pkg/cmd/betasession.go` | User-facing session creation |
| `ant auth login` | `pkg/cmd/cmd_auth.go` | OAuth or API-key login flow, writes profile config |

`ant beta:worker poll` flags (from `worker.go`): `--environment-id`, `--environment-key`, `--worker-id`, `--base-url`, `--on-work`, `--workdir`, `--unrestricted-paths`, `--max-idle`, `--log-format`.

### How it authenticates

Two paths, depending on the command:

- **Worker commands** (`ant beta:worker poll`, `ant beta:worker run`) authenticate with the **environment key**. The SDK helpers attach it as a per-request `Authorization` bearer and **delete** the parent client's `X-Api-Key` header on every helper-issued call (see `bearerReqOpts` in `lib/environments/poller.go`). This is by design: "the client itself carries no credential and only needs an optional base URL" for the worker path. The key is read from `ANTHROPIC_ENVIRONMENT_KEY` env or `--environment-key` flag.
- **Everything else** (sessions, environments, agents, messages, files) authenticates with the org API key via `ant auth login` (writes a profile with org/workspace IDs and a refresh token), OAuth, or `ANTHROPIC_API_KEY` env. There are also workload-identity-federation and AWS IAM auth paths introduced in v1.5.0 and v1.10.0.

The CLI also redacts API-key headers from debug logs (v1.7.1).

### Polling shape

**It is a long-running poller, not one-shot.** The default mode runs forever. The SDK has a `Drain` option to exit when the queue is empty (used for the webhook-triggered shape). The non-blocking single-poll is also exposed via `BlockMs = param.Null[int64]()`.

The polling backoff is jittered exponential, capped at 60 s, reset on every successful poll. On `4xx` (other than 408, 429) the poller exits with an error rather than burning the queue.

A subtle but important detail from the SDK comment: "A WorkPoller is NOT safe for concurrent use. All methods must be called from a single goroutine." A multi-worker fleet runs N separate WorkPollers, not one shared one.

### CLI vs library split

This is the critical finding for the in-process embedding hypothesis: **the polling and worker logic lives in the SDK, not the CLI**.

- `anthropic-cli/pkg/cmd/worker.go` is ~250 lines of urfave/cli flag glue. It constructs `environments.NewEnvironmentWorker(...)`, calls `.Run(ctx)`, or constructs `environments.NewWorkPoller(...)` and execs the `--on-work` script per item. That's it.
- `anthropic-sdk-go/lib/environments/poller.go` (~600 lines, hand-written, with a substantial package doc) is the actual claim/ack/stop loop.
- `anthropic-sdk-go/lib/environments/worker.go` (~700 lines) is the `EnvironmentWorker` composition: poller plus skill download plus tool runner plus heartbeat plus force-stop.
- Tool implementations live in `anthropic-sdk-go/tools/agenttoolset`.

So if we want "polling in coderd", we import `github.com/anthropics/anthropic-sdk-go` and call `environments.NewWorkPoller(...)` directly. There is no value in pulling the CLI as a library; it would only add the urfave/cli dependency tree and the Bubbletea TUI we don't need.

### Language and license

Go 1.25, MIT. The SDK (`anthropic-sdk-go`) is also Go (`go 1.24`, `toolchain go1.25.8`) and MIT. Coder is on Go 1.26.4. Compatible.

Stainless codegen note: the `Beta*` types throughout are explicitly beta API surfaces gated by the `managed-agents-2026-04-01` beta header. Anthropic may break these between releases. The `lib/environments` package is the relatively stable hand-written wrapper; the underlying `Beta.Environments.Work.*` calls are codegen and more volatile.

## 3. The existing Coder PoC

### Repo facts

- Repo: https://github.com/coder/coder-anthropic-integration-poc (private).
- Status: 1 author (Kyle), described as "Kyle testing Anthropic's sandbox stuff with Coder". Last update 2026-06-03. Single commit on `main`. No tests, no CI.
- Language: HCL (Terraform module) plus a bash spawn script.

### Repo layout

```
/
├── README.md                    end-to-end install + operate guide
├── start.sh                     the --on-work script (50 lines bash)
├── module/
│   ├── README.md
│   ├── main.tf                  the reusable Coder template module
│   └── run.sh                   coder_script rendered into the template
└── example-template/
    ├── README.md
    └── main.tf                  minimal docker workspace template using the module
```

### How it wires `ant` to Coder

The flow, traced end-to-end:

1. Operator runs `ant beta:worker poll --on-work ./start.sh` on a "worker host" outside Coder. Worker host needs the `ant` CLI, the `coder` CLI, a Coder API token for a service-account user, and the `CODER_URL`, `CODER_SESSION_TOKEN`, `CODER_TEMPLATE` env vars set.
2. `ant` claims a work item via the SDK's `WorkPoller`, then execs `start.sh` with `ANTHROPIC_{SESSION_ID, ENVIRONMENT_KEY, WORK_ID, ENVIRONMENT_ID, BASE_URL}` in env.
3. `start.sh` derives a deterministic workspace name from the session ID (`ant-<slug>`, 32-char cap), then calls `coder create $NAME --template $CODER_TEMPLATE --yes --parameter anthropic_session_id=... [4 more]`. This is the **job-to-workspace mapping line**, in `start.sh` lines 36 to 43 of HEAD.
4. The Coder template includes the `module/main.tf` from this repo. That module declares 5 **ephemeral** `coder_parameter`s (one per `ANTHROPIC_*` value) with empty defaults. On workspace start, the module:
   - Wires each parameter into a `coder_env` resource on the workspace agent.
   - Renders `run.sh` into a `coder_script` that runs on agent start. `run.sh` `cd`s to `working_directory` and execs the configured command (default `ant beta:agent run`, see "Notable gap" below).
5. Inside the workspace, the agent runs `ant beta:worker run` (or whatever `var.command` resolves to). That picks up the session via the env vars and starts the per-item flow: download skills, run tools, heartbeat, post results. When the session ends, `run.sh`'s `trap` touches `/tmp/anthropic-session.done`.
6. Back on the worker host, `start.sh` is blocked in a loop running `coder ssh $NAME -- test -f /tmp/anthropic-session.done` every `${ANTHROPIC_POLL_SECONDS:-5}` seconds. When the file appears, `start.sh` falls through, the `EXIT` trap fires, and it calls `coder delete --yes $NAME`. Workspace destroyed.

### Job-to-workspace mapping (exact location)

- `start.sh:36-43`: the `coder create` invocation. Slug derivation at `start.sh:23-25`.
- `module/main.tf:43-83`: the five ephemeral parameter declarations.
- `module/main.tf:96-115`: `coder_env` resources that propagate the parameters into the agent's environment.
- `module/main.tf:118-128`: the `coder_script` resource that renders `run.sh`.
- `module/run.sh:13-20`: the inner exec of `${command}` (which is `ant beta:worker run`, modulo the gap below). `trap 'touch $DONE_FILE' EXIT` on line 8 is the back-channel signal.

### What is hardcoded, hand-waved, or missing

- **Service-account model.** All workspaces are owned by one service-account user (`anthropic-worker`). Multi-tenant Coder deployments cannot map an Anthropic session to a specific human Coder user. The session's `metadata` field is the natural carrier but the PoC ignores metadata entirely.
- **Sensitive parameter leakage.** The README flags this: `coder_parameter` has no `sensitive` flag, so the environment key shows up in the workspace create form, in the parameter audit log, and in API responses. The repo says: "a `sensitive = true` flag on `coder_parameter` would be ideal for storing these values encrypted and redacting them from API responses". Confirmed by inspecting `coder/terraform-provider-coder`; only `coder_agent.token` and `coder_external_auth` have `Sensitive: true` today.
- **Notable gap: stale inner command default.** `module/main.tf:9` declares `default = "ant beta:agent run"`, and the README's "How it fits together" diagram says the same. But the actual CLI in v1.12.2 (and as far back as v1.9.0) registers the command as `ant beta:worker run`, not `ant beta:agent run`. Either the PoC was written against a pre-rename internal build, or it has been broken since the rename. The example template in `example-template/main.tf` does not override `var.command`, so a fresh checkout would invoke a non-existent CLI command.
- **No retries on workspace create.** `coder create` failure kills the whole `start.sh`, which kills the work item handler. The work item gets force-stopped (because `start.sh` has a `trap cleanup EXIT`), but the SDK's poller continues. The README implies "set, forget, run under systemd" but does not document what failures look like.
- **No graceful shutdown.** `start.sh` does not propagate SIGTERM into the workspace. If the worker host is terminated mid-session, the `coder delete` runs but the Anthropic side sees a heartbeat timeout, not a graceful stop.
- **Polling over `coder ssh test -f`.** Five-second polling against a tunneled SSH command, holding open the SSH lease, for what is essentially a "session done" signal. The Coder agent already has rich lifecycle hooks (`coder_script` stop, `coder_app` health checks, agent lifecycle stages); none are used. Cheap to fix, but illustrative of the PoC's "duct tape" character.
- **Working directory and skill download.** The module hardcodes `/home/coder/work` as a suggested working directory. Inside that, the SDK will create `skills/<name>/` per session. There is no cleanup; the skills persist across workspaces (when the template uses a persistent home), which the docs do not flag but which may be a privacy issue if `$HOME` is shared.
- **One environment per worker host.** The PoC's `start.sh` is per-environment. There is no fan-out for an org that wants to expose multiple environments (per-team isolation per the security model). Operators would need to run multiple worker processes.
- **No `--metadata` flow-through.** Anthropic intends `session.metadata` to carry session-specific context (input file SHA, target repo, user identity). The PoC's spawn script does not read it. The session metadata API call exists on the worker side: "Your spawn script or `--on-work` handler reads that metadata from the claimed work item ... and stages the files into the working directory before tool execution begins". PoC does not.

## 4. Validation of the user's hypothesis

| # | Claim | Verdict | Evidence |
|---|---|---|---|
| 1 | "Anthropic self-hosted sandboxes just need a compute environment to deploy 'their agent'" | ⚠️ Partially accurate | The agent (Claude plus the agentic loop) **stays in Anthropic's control plane**. What you deploy is an **environment worker** that executes tools on behalf of the agent. Docs: "Self-hosted sandboxes keep the orchestration on Anthropic's side but move tool execution into infrastructure you control". The user's phrasing "their agent" gets the runtime location wrong. |
| 2 | "That agent phones home back to Anthropic" | ⚠️ Partially accurate | Directionally correct (worker dials out), but the framing of "phones home" implies the worker is the agent. In reality the worker long-polls Anthropic for work items and the per-session SSE event stream. No agent code lives on your side. |
| 3 | "The Anthropic control plane + agentic loop handles all bi-directional communication with the agent" | ✅ Accurate | The control plane drives the loop. Tool calls flow agent-to-worker via SSE on the session event stream; tool results flow worker-to-agent as `user.tool_result` events. The worker is a transport-and-execution adapter, not a participant in the loop. |
| 4 | "To work in Coder, we'd use the `ant` CLI to poll for a new job request (a request for a compute environment)" | ⚠️ Partially accurate | The CLI does poll, but: (a) the polling logic is in the SDK, the CLI is a thin wrapper; (b) the "job" is a **session work item**, not a "request for a compute environment". The compute environment exists *before* the work item is delivered; the worker is already running and just claims the next item assigned to its queue. |
| 5 | "On each request we create a new workspace from a template that has the agent" | ⚠️ Partially accurate, with caveats | This is one valid implementation (the per-session sandbox model in the docs) and is what the PoC does. The PoC's "agent in the template" is actually `ant beta:worker run` running inside the workspace, claiming the same session that the outer poller just released. The "agent" lives outside; what's in the template is the **inner runner** that executes tools. |
| 6 | "The PoC at coder/coder-anthropic-integration-poc does roughly that" | ✅ Accurate | The PoC implements exactly the per-session sandbox pattern with Coder workspaces as the sandbox unit. See section 3 for the exact lines. |
| 7 | "Improvement: do the `ant` CLI polling loop inside coderd (import the CLI, run it in-process) for a more first-class experience" | ❌ Wrong in detail, plausible in spirit | Four issues: (a) you would import the **SDK** (`anthropic-sdk-go/lib/environments`), not the CLI; (b) "the polling loop" is per-environment, and an org will have many environments (per-team isolation per the security model), so coderd has to manage a fleet of pollers, not a single loop; (c) you still need to decide **which Coder user owns the resulting workspace** (the work item carries no Coder identity); (d) environment-key storage in coderd is non-trivial because the keys are bearer tokens with org-wide blast radius if leaked. The spirit (first-class integration) is reasonable; the mental model that "it's just one loop to embed" understates the design work. See section 5. |

## 5. "Polling loop in coderd" architecture analysis

This section assumes we decide to embed. I argue against doing so as the first step in section 8, but let's enumerate what it would look like.

### Where it would live

- **Most coherent home: `enterprise/` or a new top-level package** like `enterprise/anthropicpoller/`. Reasons:
  - It is a paid, premium integration, not core Coder functionality. Open-source Coder should not depend on Anthropic SDK transitively.
  - The poller manages **identity and secrets**, which couples to enterprise auth (groups, org users, audit, scim).
  - Coder has prior art for "external-system to Coder workspace" daemons: `coderd/aibridged/` is a daemon that proxies AI provider traffic, and `provisionerd` is a daemon that owns long-running infrastructure work. The poller belongs in that company.
- **Not in `coderd/x/chatd/`.** Chatd is the in-Coder chat interface for users to talk to Claude / other models through Coder. Self-hosted sandboxes are about Anthropic-driven sessions originating outside Coder. Different direction, different lifecycle, different audit story.

### Integration points

The work the daemon must do, mapped to existing coderd surfaces:

| Concern | Coder surface | What's needed |
|---|---|---|
| Resolve the session owner | `httpmw.APIKey`, `database.User` | Map Anthropic session metadata (or env key tagging) to a Coder user. New: a config for "this environment is owned by user X" or "extract user from `session.metadata.coder_user`". |
| Pick the template | `database.Templates`, `templateversions` | Either bind one template per Anthropic environment, or derive from `session.metadata.template`. Bind to the active template version. |
| Inject parameters | `coderd/workspacebuilds.go`, `provisionersdk` rich parameters | Replace the 5 ephemeral parameters with **non-rendered**, agent-only env injection. Today the PoC uses ephemeral parameters because there is no other tool. coderd could do this directly without a `coder_parameter` round-trip. |
| Create the workspace | `api.postWorkspacesByOrganization` (`coderd/workspaces.go`) | Internal call, not over HTTP. Reuse the create build code path; do not shell out to `coder` CLI. |
| Hand off the agent token | `provisionerdserver` build of the resources, `coder_agent.token` | Existing; the agent gets its token via the agent init script. |
| Stream session lifecycle | `coderd/aitasks` (Task API already exists), `agentapi` | A "Task" in coderd already has a state machine; map Anthropic session events to task state. |
| Detect "done" | Agent lifecycle (`stop` script in coder_script), workspace stop transition | Avoid the `coder ssh test -f` poll; use the agent's natural stop signal. |
| Tear down | Workspace delete transition | Reuse, but make it configurable: some users will want the workspace to persist for inspection. |
| Audit | `coderd/audit` | Every workspace create from a poll must be audited with the Anthropic session ID as a correlation field. |

### In-process vs external poller

**Pros of in-process:**

- One binary to deploy, scale, and observe.
- Direct DB and RBAC access; no Coder API token to manage; no service-account user needed.
- Lower latency on workspace create (no HTTP round-trip from external poller to coderd).
- Easier per-org and per-user identity flow because we own the auth context.

**Pros of external sidecar (the PoC pattern, evolved):**

- Coder's blast radius on Anthropic auth changes is zero; an SDK upgrade is a sidecar redeploy, not a coderd redeploy.
- Crashed poller does not crash coderd.
- The poller can run somewhere with **direct egress to Anthropic** even if coderd is air-gapped to a private API endpoint.
- Beta API surface stability is bad; the SDK pins `managed-agents-2026-04-01` as the beta header. If Anthropic ships `2026-09-01` with breaking changes, an external sidecar can roll independently.
- Existing PoC plus a real binary in `coder-anthropic-poller` is days of work, not weeks.

**Cons of in-process:**

- coderd's binary picks up `anthropic-sdk-go` plus its transitive deps (`go.opencensus.io`, OpenTelemetry, AWS SDK for SigV4, GCP auth, `modelcontextprotocol/go-sdk`, etc.). That is a lot of new code.
- Beta-tagged surfaces baked into coderd's release cadence.
- Polling per environment per org multiplies goroutine count; each long-poll holds a TCP connection. Single-coderd deployments are fine; HA deployments need leader election.
- Worker restart mid-session: the in-process worker holds the SDK's heartbeat goroutine. If coderd restarts, the session's lease expires after `reclaim_older_than_ms` (default 5000 ms) and another coderd instance can reclaim it. But the in-flight tool call dies, and the per-session workspace orphaned. This needs a reaper.

### Concurrency model

For a multi-tenant Coder cluster:

- One poller per `(coderd_replica, environment_id)` pair would over-claim; environments would see N workers per replica.
- Better: one poller per `environment_id`, with leader election across coderd replicas. The SDK's `WorkerID` field is meant for exactly this kind of attribution; deduplication on the server side is reclaim-based, not lease-based, so two pollers claiming the same environment is technically safe but operationally noisy.
- Per-org isolation: the security model wants one environment per trust boundary. Practically this means N environments per Coder deployment (one per Coder organization, say), so N pollers per deployment.
- User identity flow: the cleanest is to tag the environment as "this environment belongs to user X" via Anthropic Console naming, and have coderd read `environment_id -> coder_user_id` from a config table. The alternative (user identity in `session.metadata`) requires every session creator to set it, which is fragile.

### Restart-in-the-middle behavior

Three windows:

1. **Between poll and ack.** The work is leased to us. On restart, the lease times out (default 5 s post-poll, configurable). Another instance picks it up. Lost: nothing.
2. **Between ack and first tool result.** Workspace is being created. On restart, the workspace create is in-flight in the provisioner. The Anthropic side will heartbeat-timeout. Cleanup needed: when coderd comes back, scan for workspaces tagged with an active Anthropic session that has no recent heartbeat, and either resume (re-attach the agent to the session) or delete.
3. **Mid-session.** Agent is running, tools are firing. On restart, the agent in the workspace is still talking to Anthropic (the worker process is *inside* the workspace, not in coderd). So the in-process coderd poller is not actually involved in the per-session loop in the PoC architecture. **This is a key insight**: the PoC moves the per-session loop into the workspace, so coderd's role is only "claim work, spawn workspace, observe completion".

### Observability hooks

Minimum viable:

- `coderd_anthropic_poller_active{environment_id}` gauge (1 if the poller is running).
- `coderd_anthropic_poll_seconds{environment_id, result}` histogram (`result=claimed|empty|error`).
- `coderd_anthropic_sessions_active{environment_id, coder_user_id}` gauge.
- `coderd_anthropic_session_duration_seconds{environment_id}` histogram.
- Audit events: `anthropic.session.received`, `anthropic.workspace.created`, `anthropic.workspace.deleted` with the Anthropic session ID and work ID.
- Slog with `slog.With("anthropic_session_id", ..., "work_id", ...)` from claim onward.

## 6. Alternative architectures to weigh

### A. External sidecar daemon calling coderd HTTP API (the PoC's apparent model, hardened)

The PoC is one bash script. A v0 sidecar would be a small Go binary that imports `anthropic-sdk-go/lib/environments`, runs `WorkPoller` with `Drain=false`, and calls Coder's existing workspace create API per claim. It owns the polling concurrency, retry logic, dead-session cleanup, and a small SQLite or Redis state for "session ID -> workspace ID -> done?". The sidecar holds its own Coder API token (one per Anthropic environment, per Coder org) and its own Anthropic environment key.

**Pick when:** we want to ship something within weeks, we want the Anthropic SDK to upgrade independently of coderd's release cadence, we want a clean "this works the same on Coder OSS and Enterprise" story.

**Don't pick when:** the per-org user identity story requires close coupling with coderd's auth (groups, scim, org membership), or when we want to expose Anthropic session state in the Coder UI alongside other tasks.

### B. Provisioner-style worker daemon registered to coderd

Borrow the `provisionerd` pattern: a separate process that registers with coderd over the websocket protocol, runs the Anthropic SDK poll loop, and receives "create workspace" instructions back from coderd. Coderd owns the auth, RBAC, audit; the daemon owns the Anthropic-side state.

**Pick when:** we want the daemon model but want coderd to be the source of truth for which Coder user owns each session, and we want the daemon to be deployable in a network zone different from coderd (so it can reach Anthropic while coderd lives behind a firewall).

**Don't pick when:** the daemon protocol design effort outweighs the integration value. The protocol must carry: "claim succeeded", "session metadata", "tool results", "session done". That's a non-trivial RPC surface.

### C. Coder external-auth plus custom workspace agent running `ant` inside the workspace

A user-initiated flow: a developer opens a workspace from a template with the Anthropic module pre-installed, and the workspace agent runs `ant beta:worker poll --environment-id $ENV --environment-key $KEY` inside the workspace. The environment key comes from Coder's external-auth integration (a new external-auth provider type for Anthropic), so the user authorizes once and the key flows to the agent securely.

**Pick when:** the use case is "let a developer pair with a Claude-driven assistant inside their workspace". Different model: the developer is in the loop, the workspace is durable, the agent is a sidekick. This is closer to the existing AI Task pattern.

**Don't pick when:** the use case is "Anthropic orchestrates a fleet of one-shot sandboxes triggered from outside Coder", which is what the user's hypothesis and the PoC describe.

### D. Anthropic webhooks into coderd instead of long-polling

Anthropic offers webhook-triggered workers as a documented alternative. Coderd exposes `/anthropic/webhook`, verifies the standard-webhooks signature (the SDK has a `webhooks.Unwrap` for this), and on `session.status_run_started` starts a one-shot session handler (`work.poller(drain=true)`). The handler creates a workspace, hands off to it, and exits.

**Pick when:** we want minimum idle resource usage on coderd's side (no long-poll goroutines), and we are comfortable depending on a public webhook endpoint reachable from Anthropic.

**Don't pick when:** the deployment is air-gapped or behind a corporate firewall (no inbound), or when we want sub-second claim latency (long-poll is faster than webhook-then-poll).

## 7. Open questions and unknowns

- **Per-org environment-key issuance.** The docs say keys are generated only in the Anthropic Console UI, even when the environment was created via API. Is there a programmatic key-rotation path coming, or do operators have to click in the Console for every new Coder org? Unclear from public docs.
- **Multi-environment quota.** Are there limits on the number of self-hosted environments per Anthropic workspace? The rate limits doc only lists API rate limits (300 creates/min, 600 reads/min). Per-environment work-queue depth is also undocumented.
- **Session metadata schema for user identity.** Anthropic does not document a canonical "this session belongs to user X" metadata key. We would need to define our own convention.
- **Skill download lifecycle in long-lived workspaces.** Skills are downloaded to `<workdir>/skills/<name>/`. The docs do not describe what happens when two sessions in the same workspace need different versions of the same skill, or when the agent's skill set changes between sessions. Will the SDK overwrite, version, or merge?
- **`reclaim_older_than_ms` semantics in HA.** If two coderd replicas both call `work.poll` against the same environment, the SDK reclaims work items older than the configured threshold. Is reclaim idempotent on the server side, or can two replicas both end up "owning" the same session for a window? Tests would tell.
- **Cost model.** Anthropic Managed Agents pricing is per-token; the self-hosted sandbox is free for the compute (we own it). But there is no published model-call quota at the environment level. An accident in the agent loop (e.g., infinite tool-call loop) could be expensive.
- **`ant beta:worker run` vs `ant beta:agent run`.** The PoC's default `var.command = "ant beta:agent run"` does not match the current CLI. We need to confirm: was this ever a real command, or has the PoC been broken since v1.9.0? Run `ant --help` against v1.12.2 to confirm.
- **MCP server reach from inside the sandbox.** Docs say MCP tunnels are orthogonal to self-hosted sandboxes. Inside a Coder workspace running the worker, does the worker reach MCP servers via the workspace's network policy or via the tunnel? Both should work; the operational story is unclear.

## 8. Recommended next steps

Opinionated, in order:

1. **Spike the SDK end-to-end in 1 day, without Coder.** Run `ant beta:worker poll` on a laptop. Confirm the actual command set in v1.12.2 (especially that `ant beta:worker run` is the inner-runner name, since the PoC uses `ant beta:agent run`). Get a feel for poll latency, claim semantics, and what session events look like. Output: a written confirmation or correction of the PoC's inner-command default.

2. **Fix the PoC (one PR) before deciding on architecture.** Concrete bugs to address: (a) `module/main.tf` `var.command` default if it is indeed stale; (b) drop the `coder ssh test -f` poll in favor of the agent's natural stop signal; (c) thread `session.metadata` through `start.sh` so the workspace can know which user this is for. This buys us a working baseline to compare anything else against.

3. **Decide the architecture explicitly: sidecar or in-process.** Write a one-page decision doc with the team. My vote is **sidecar first** (Section 6A), repo `coder/coder-anthropic-poller`, single Go binary that imports `anthropic-sdk-go/lib/environments`, calls coderd's HTTP API. Reasons: ships in weeks; SDK upgrades decoupled from coderd release; can run in a different network zone from coderd; matches the PoC's mental model so the migration path is incremental.

4. **Design the Coder-side session-to-user mapping.** Two paths to compare on paper:
   - Per-environment static binding: `coderd` config maps `environment_id` to `coder_user_id` and `template_id`.
   - Per-session metadata binding: `session.metadata.coder_user_id` and `session.metadata.coder_template_id`, validated server-side.
   The latter is more flexible but requires every session creator to set the metadata correctly. Pick before writing code.

5. **Add a `sensitive` flag to `coder_parameter`** (or accept that we propagate via a different mechanism). The current PoC leaks the environment key into the workspace create form. Even with a sidecar that bypasses parameters, the next person to copy-paste the PoC will hit this. This is a small, useful unblock for the broader templating ecosystem.

6. **Prototype the in-process variant against the SDK behind a build tag.** Behind `//go:build experimental_anthropic`, import `anthropic-sdk-go/lib/environments`, wire a single-environment poller into coderd, and measure: binary size delta, transitive dep churn, restart story under chaos. Only after this is the in-process embedding a defensible default.

7. **Surface in the AI Tasks UI.** Whatever architecture we pick, the user-visible artifact should be a "Task" (`coderd/aitasks.go`). The user sees a Claude-driven task, with status, logs, and a "stop" button. The plumbing underneath can be sidecar, in-process, or webhooks.

## Appendix: raw source notes

### Anthropic docs

- **https://docs.anthropic.com/en/managed-agents/self-hosted-sandboxes** (mirror of `platform.claude.com`). Confirms the work-queue model. Pivotal quote: "the `self_hosted` environment acts as a work queue: when a session is assigned to it, Anthropic enqueues the session as a work item". Always-on vs webhook-triggered choice, in-process vs sandbox-per-session choice, SDK helper layers (`EnvironmentWorker`, `work.poller`, `tool_runner`).
- **https://docs.anthropic.com/en/managed-agents/self-hosted-sandboxes-security**. Shared responsibility model. "The environment service key ... authorizes polling your environment's work queue and submitting results back to sessions." Treat like a database password.
- **https://docs.anthropic.com/en/managed-agents/reference**. Confirms the CLI flags for `ant beta:worker`: `--environment-id`, `--environment-key`, `--workdir`, `--on-work`, `--unrestricted-paths`, `--max-idle`, `--log-format`. Lists session/agent/span/system event types. Rate limits: 300 creates/min, 600 reads/min per org. Beta header: `managed-agents-2026-04-01`.
- **https://docs.anthropic.com/en/managed-agents/sessions**. Session creation takes `agent` plus `environment_id`; the session waits in queue if no worker is connected.

### anthropic-cli (GitHub)

- **Repo:** `anthropics/anthropic-cli` at HEAD `eaa75ac813`. Go 1.25, MIT, Stainless-generated.
- **`pkg/cmd/worker.go`:** the only hand-written worker code. Defines `workerPollCommand` and `workerRunCommand`. Constructs `environments.NewEnvironmentWorker` or `environments.NewWorkPoller` from the SDK. ~250 lines total.
- **`pkg/cmd/betaenvironmentwork.go`:** raw API commands (poll/ack/heartbeat/stop/stats), all marked "called automatically by the pre-built environment worker ... included here as a reference; you do not need to invoke them directly".
- **`CHANGELOG.md`:** v1.9.0 (2026-05-19) "Add support for self-hosted sandboxes in CMA with sandbox helpers". v1.12.2 is latest as of 2026-06-24.
- **`go.mod`:** depends on `anthropic-sdk-go v1.51.1`, plus `urfave/cli/v3`, `charmbracelet/bubbletea` (for the interactive JSON explorer).

### anthropic-sdk-go (GitHub)

- **Repo:** `anthropics/anthropic-sdk-go` at HEAD `2978fa57`. Go 1.24, toolchain 1.25.8, MIT.
- **`lib/environments/poller.go`:** `WorkPoller` with `Next/Current/Err/Close` plus a Go 1.23 `All()` range-over-func iterator. Embeds the `managed-agents-2026-04-01` beta header. Explicit per-request `Authorization: Bearer` with `X-Api-Key` cleared. "A WorkPoller is NOT safe for concurrent use".
- **`lib/environments/worker.go`:** `EnvironmentWorker` composition that ties the poller to the skill download, the session tool runner, and heartbeat or force-stop. `.Run()` for long-running and `.HandleItem()` for one-shot.
- **`tools/agenttoolset`:** `BetaAgentToolset20260401` constructs the standard tool list (`bash`, `read`, `write`, `edit`, `glob`, `grep`).
- **Cadence:** v1.52.0 (2026-06-24) is latest; releases roughly weekly, primarily Stainless-generated, the `lib/environments` package is hand-written with substantial commentary.

### coder/coder-anthropic-integration-poc (GitHub, private)

- **Repo:** `coder/coder-anthropic-integration-poc` at HEAD `7bce3348`. Created 2026-06-02, last touched 2026-06-03. Single contributor. No tests, no CI, single commit on `main`.
- **`README.md`:** end-to-end install guide. Explicitly calls out the missing `sensitive` flag on `coder_parameter` and the 5-second `coder ssh` poll.
- **`start.sh`:** 50 lines. The `--on-work` script. Reads 5 `ANTHROPIC_*` env vars, runs `coder create` with them as `--parameter`s, polls for a done-file over `coder ssh`, runs `coder delete --yes`.
- **`module/main.tf`:** 5 ephemeral `coder_parameter`s, 5 `coder_env`s, a `coder_script` rendered from `run.sh`. The `var.command` default is `"ant beta:agent run"`, which does not match the current `ant beta:worker run` subcommand name.
- **`module/run.sh`:** runs at agent start, `cd`s to working_directory, execs `${command}`, `trap`s an exit to touch `/tmp/anthropic-session.done`.
- **`example-template/main.tf`:** a minimal Docker-provider workspace template that uses the module.

### coder/coder (local checkout, read-only inspection)

- `coderd/aitasks.go`: existing AI Task API surface (`POST /api/v2/tasks/{user}`). Tasks are workspaces with a prompt input and agentapi integration. Natural integration point for surfacing Anthropic sessions.
- `coderd/aibridged/`: existing daemon that proxies AI provider traffic. Architectural precedent for "external-system to Coder" daemon.
- `coderd/x/chatd/`: existing in-Coder chat daemon. Not the right home for self-hosted sandboxes (different direction of traffic).
- `coderd/x/skills/`: existing per-agent skills surface. Note collision with Anthropic "skills" terminology; the two are unrelated but the name overlap will confuse readers.
- `provisionersdk/proto/`: shows that adding a new "agent env injection" path would require provisioner protocol work; the PoC's ephemeral-parameter shortcut avoided this at the cost of leaking the env key.
- `coder/terraform-provider-coder` `provider/agent.go`: only `coder_agent.token` is marked `Sensitive: true`. `coder_parameter` does not have a sensitive flag today, confirming the PoC's caveat.
