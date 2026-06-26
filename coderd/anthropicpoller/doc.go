// Package anthropicpoller implements the coderd side of the Anthropic
// self-hosted sandboxes integration.
//
// One Poller runs per Coder organization that has Anthropic self-hosted
// sandbox configuration. It long-polls the org's Anthropic environment
// work queue using github.com/anthropics/anthropic-sdk-go/lib/environments,
// then hands each claimed work item to a WorkspaceDispatcher. The
// dispatcher is responsible for resolving the target Coder user from the
// work item's metadata, creating a workspace owned by that user, and
// tearing it down when the session is done.
//
// The poller does not heartbeat the work lease itself. That responsibility
// is owned by the inner `ant beta:worker run` process running inside the
// spawned workspace, which takes over via the SDK's
// ExpectedLastHeartbeat handoff once the workspace agent boots.
//
// See coderd/anthropicpoller/DESIGN.md for the full design and the open
// PoC decisions.
package anthropicpoller
