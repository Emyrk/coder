package anthropicpoller

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
)

// WorkspaceDispatcher is the boundary between the poller and the coderd
// workspace machinery. Implementations spawn one Coder workspace per
// claimed work item, inject the ANTHROPIC_* environment variables the
// inner runner needs, and block until the session reaches a terminal
// state. Workspace cleanup (delete or stop) is the implementation's
// responsibility.
//
// A WorkspaceDispatcher only needs to be safe for sequential calls from a
// single Poller goroutine. The poller serializes work items in the PoC;
// concurrency support can be added once the dispatch path is fleshed out.
type WorkspaceDispatcher interface {
	// Dispatch handles one claimed work item end to end:
	//
	//  1. Resolve the target Coder user from work.Metadata.
	//  2. Create a workspace owned by that user.
	//  3. Wait until the Anthropic session reaches a terminal state.
	//  4. Delete or stop the workspace per the deployment policy.
	//
	// Returning a non-nil error is logged by the poller but does not stop
	// the poll loop; the next iteration will claim the next work item.
	Dispatch(ctx context.Context, work *anthropic.BetaSelfHostedWork) error
}

// NoopDispatcher records every work item it received but never creates
// workspaces. Intended for tests and for early bring-up before the real
// workspace plumbing is wired.
type NoopDispatcher struct {
	// Received accumulates the work items passed to Dispatch, in claim
	// order. Tests inspect this slice to assert what arrived.
	Received []*anthropic.BetaSelfHostedWork
}

// Dispatch implements WorkspaceDispatcher.
func (d *NoopDispatcher) Dispatch(_ context.Context, work *anthropic.BetaSelfHostedWork) error {
	d.Received = append(d.Received, work)
	return nil
}
