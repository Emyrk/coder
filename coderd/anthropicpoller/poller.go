package anthropicpoller

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/lib/environments"
	"golang.org/x/xerrors"

	"cdr.dev/slog/v3"
)

// Poller is the per-Coder-organization Anthropic work poller. One Poller
// instance owns one long-poll goroutine against one Anthropic environment.
//
// The underlying SDK WorkPoller is not safe for concurrent use, so a
// Poller must have at most one Run call active at a time.
type Poller struct {
	cfg        OrgConfig
	client     anthropic.Client
	dispatcher WorkspaceDispatcher
	logger     slog.Logger
}

// New constructs a Poller. The provided client does not need to carry an
// API key; the environments helpers authenticate every request with the
// EnvironmentKey from cfg.
func New(cfg OrgConfig, client anthropic.Client, dispatcher WorkspaceDispatcher, logger slog.Logger) (*Poller, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if dispatcher == nil {
		return nil, xerrors.New("anthropicpoller: dispatcher is required")
	}
	return &Poller{
		cfg:        cfg,
		client:     client,
		dispatcher: dispatcher,
		logger: logger.Named("anthropicpoller").With(
			slog.F("org_id", cfg.OrgID),
			slog.F("environment_id", cfg.EnvironmentID),
		),
	}, nil
}

// Run blocks, long-polling the Anthropic environment for work items and
// dispatching each claimed item to the configured WorkspaceDispatcher.
// Run returns nil on context cancellation (normal termination) and a
// non-nil error if the SDK poller hits a non-retryable failure.
//
// The poller is single-threaded: claim one item, dispatch synchronously,
// claim the next. The SDK's deferred Stop fires on the next Next call.
func (p *Poller) Run(ctx context.Context) error {
	wp := environments.NewWorkPoller(ctx, p.client, environments.WorkPollerOptions{
		EnvironmentID:  p.cfg.EnvironmentID,
		EnvironmentKey: p.cfg.EnvironmentKey,
		// Logger left nil; SDK falls back to slog.Default(). A bridge into
		// cdr.dev/slog can replace this once we decide we want SDK-internal
		// records in the project's log stream.
	})
	defer wp.Close()

	p.logger.Info(ctx, "anthropic poller starting")
	defer p.logger.Info(ctx, "anthropic poller stopped")

	for wp.Next() {
		work := wp.Current()
		if work == nil {
			continue
		}
		wlog := p.logger.With(
			slog.F("work_id", work.ID),
			slog.F("session_id", work.Data.ID),
		)
		wlog.Info(ctx, "claimed work item")

		if err := p.dispatcher.Dispatch(ctx, work); err != nil {
			wlog.Error(ctx, "dispatch failed", slog.Error(err))
			// Continue polling. The SDK's deferred Stop fires on the next
			// Next call regardless of dispatch success.
		}
	}

	if err := wp.Err(); err != nil {
		return xerrors.Errorf("anthropic poller: %w", err)
	}
	return nil
}
