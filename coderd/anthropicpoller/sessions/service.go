package sessions

import (
	"github.com/anthropics/anthropic-sdk-go/option"
	"golang.org/x/xerrors"

	"cdr.dev/slog/v3"
	"github.com/coder/coder/v2/coderd/anthropicpoller"
	"github.com/coder/coder/v2/coderd/database"
)

// Service owns the dependencies the Anthropic upstream HTTP handlers
// need: the per-org config (for the EnvironmentID we stamp on session
// creation), the Coder database (for user_secrets lookups), and a slog
// Logger.
//
// A Service is safe for concurrent use because all of its state is
// read-only after construction. Each handler builds a one-shot
// [anthropic.Client] from the calling user's stored API key.
type Service struct {
	// Config is the same per-org configuration the poller uses. The
	// sessions handler only reads Config.OrgID (for response logging
	// and route guarding) and Config.EnvironmentID (stamped on every
	// session created through this service).
	Config anthropicpoller.OrgConfig

	// DB is the Coder database, used to look up the calling user's
	// ANTHROPIC_API_KEY user secret.
	DB database.Store

	// Logger receives structured events for handler observability.
	Logger slog.Logger

	// ClientOptions are appended to every Anthropic SDK client this
	// service constructs. Tests inject [option.WithBaseURL] to point
	// the SDK at an httptest server; production typically leaves this
	// empty.
	ClientOptions []option.RequestOption
}

// New validates the supplied dependencies and returns a ready Service.
//
// Returns an error if Config is invalid or DB is nil. The Logger is
// optional; New does not touch slog.Default to keep the call cheap.
func New(cfg anthropicpoller.OrgConfig, db database.Store, logger slog.Logger, opts ...option.RequestOption) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, xerrors.Errorf("validate config: %w", err)
	}
	if db == nil {
		return nil, xerrors.New("sessions: db is required")
	}
	return &Service{
		Config:        cfg,
		DB:            db,
		Logger:        logger,
		ClientOptions: opts,
	}, nil
}
