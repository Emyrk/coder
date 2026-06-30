// Package sessions implements the upstream HTTP handlers for the
// Anthropic self-hosted sandbox integration. It is a sibling concern
// to the work poller: where the poller claims work that Anthropic has
// queued for the environment, this package lets Coder users *create*
// that work in the first place by calling the Anthropic platform API
// on their behalf.
//
// Two endpoints land in coderd:
//
//   - GET  /api/v2/anthropic/{organization_id}/agents/{userid}
//     Returns the list of Anthropic agents the user's API key can see,
//     so the UI can render a picker before session creation.
//
//   - POST /api/v2/anthropic/{organization_id}/sessions/{userid}
//     Creates an Anthropic session bound to the org's environment and
//     the selected agent, stamping reserved Coder metadata on the way
//     through.
//
// Each Coder user must store an Anthropic API key as a user secret
// named [AnthropicAPIKeySecretName]. The poller's per-org environment
// key is a separate credential used only for the work queue; nothing
// in this package reads it.
package sessions
