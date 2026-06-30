package codersdk

import (
	"time"

	"github.com/google/uuid"
)

// AnthropicAPIKeySecretName is the well-known user-secret name that
// holds a Coder user's Anthropic platform API key. The Anthropic
// session-create and agent-list endpoints look the secret up by this
// exact name; the Settings -> Anthropic page writes it under the same
// name so the two paths stay in sync.
//
// The name matches the SDK's ANTHROPIC_API_KEY env var so the same
// secret can also be injected into workspaces by the secret machinery
// later without renaming.
//
//nolint:gosec // Constant holds the *name* of a secret, not its value.
const AnthropicAPIKeySecretName = "ANTHROPIC_API_KEY"

// AnthropicAgent is the Coder-facing projection of an Anthropic
// managed agent. Only the fields the Coder UI needs to render a
// picker are surfaced; everything else (system prompt, tools,
// skills, MCP servers, etc.) stays inside Anthropic.
type AnthropicAgent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Model       string    `json:"model"`
	Version     int64     `json:"version"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time"`
}

// AnthropicAgentsResponse is the response shape for the list-agents
// endpoint. A response object (instead of a bare array) gives us room
// to add pagination cursors later without breaking the wire format.
type AnthropicAgentsResponse struct {
	Agents []AnthropicAgent `json:"agents"`
}

// CreateAnthropicSessionRequest is the body for the POST create
// endpoint. Coderd injects EnvironmentID from server-side config and
// always overwrites the reserved metadata keys; the client controls
// AgentID, Title, and the non-reserved metadata.
type CreateAnthropicSessionRequest struct {
	// AgentID is the Anthropic agent the session should bind to.
	// Required. The UI selects this from [AnthropicAgentsResponse].
	AgentID string `json:"agent_id" validate:"required"`

	// Title is an optional human-readable session label that Anthropic
	// shows in its console. Coderd does not interpret it.
	Title string `json:"title,omitempty"`

	// Metadata is free-form session metadata. Coderd overwrites the
	// keys listed in [AnthropicReservedMetadataKeys]; everything else
	// is forwarded to Anthropic verbatim.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AnthropicSession is the Coder-facing projection of a newly created
// Anthropic session. Anthropic does not return a session-level secret
// on creation; the per-work-item JWT is delivered to the environment
// manager when work is claimed, which is the poller's concern.
type AnthropicSession struct {
	ID            string            `json:"id"`
	AgentID       string            `json:"agent_id"`
	EnvironmentID string            `json:"environment_id"`
	Title         string            `json:"title"`
	Metadata      map[string]string `json:"metadata"`
	CreatedAt     time.Time         `json:"created_at" format:"date-time"`

	// CoderUserID echoes the Coder user the session was created for,
	// matching the value coderd stamped into Metadata. Surfaced as a
	// dedicated field so the UI does not have to dig into Metadata.
	CoderUserID uuid.UUID `json:"coder_user_id" format:"uuid"`
}

// AnthropicReservedMetadataKeys lists the session metadata keys
// coderd always stamps. Values supplied by the client under these
// keys are silently overwritten on send; a deny-list keeps the wire
// format permissive while the integration matures.
//
// Today only coder_user_id is stamped. coder_organization_id and
// coder_template_id are reserved up-front so they can land in a
// follow-up without a wire-shape break.
var AnthropicReservedMetadataKeys = []string{
	"coder_user_id",
	"coder_organization_id",
	"coder_template_id",
}
