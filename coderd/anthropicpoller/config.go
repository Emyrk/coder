package anthropicpoller

import (
	"github.com/google/uuid"
	"golang.org/x/xerrors"
)

// OrgConfig holds the per-Coder-organization configuration the Poller
// needs to talk to one Anthropic self-hosted environment.
//
// PoC: the bootstrap path reads these from environment variables. The
// long-term storage will be an encrypted column on the organizations
// table; see DESIGN.md section 5.
type OrgConfig struct {
	// OrgID identifies the Coder organization this poller serves. Used for
	// logging and for correlating workspaces back to their source org.
	OrgID uuid.UUID

	// EnvironmentID is the Anthropic environment whose work queue we poll
	// (for example, "env_...").
	EnvironmentID string

	// EnvironmentKey is the bearer token authorizing poll, ack, and stop
	// calls against the work queue (for example, "sk-ant-oat01-..."). Treat
	// as a secret.
	EnvironmentKey string
}

// Validate returns a non-nil error if any required field is unset.
func (c OrgConfig) Validate() error {
	if c.OrgID == uuid.Nil {
		return xerrors.New("anthropicpoller: OrgID is required")
	}
	if c.EnvironmentID == "" {
		return xerrors.New("anthropicpoller: EnvironmentID is required")
	}
	if c.EnvironmentKey == "" {
		return xerrors.New("anthropicpoller: EnvironmentKey is required")
	}
	return nil
}
