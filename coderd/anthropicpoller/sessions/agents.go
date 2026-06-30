package sessions

import (
	"context"
	"errors"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"cdr.dev/slog/v3"
	"github.com/coder/coder/v2/coderd/httpapi"
	"github.com/coder/coder/v2/coderd/httpmw"
	"github.com/coder/coder/v2/codersdk"
)

// ListAgents returns the Anthropic agents the path user's stored API
// key can see. The request shape is:
//
//	GET /api/v2/anthropic/{organization_id}/agents/{userid}
//
// The org param and user param are extracted by the standard
// httpmw chain mounted on the route. The user secret named
// [AnthropicAPIKeySecretName] supplies the credential; coderd does
// not cache it across requests.
//
// 412 Precondition Failed signals "user has not configured a key
// yet"; the UI handles this by routing to Settings -> Secrets.
//
// @Summary List Anthropic agents the user can see
// @ID list-anthropic-agents
// @Security CoderSessionToken
// @Produce json
// @Tags Anthropic
// @Param organization path string true "Organization ID"
// @Param user path string true "User ID, username, or me"
// @Success 200 {object} codersdk.AnthropicAgentsResponse
// @Router /api/v2/anthropic/{organization}/agents/{user} [get]
func (s *Service) ListAgents(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	org := httpmw.OrganizationParam(r)
	user := httpmw.UserParam(r)

	if org.ID != s.Config.OrgID {
		httpapi.Write(ctx, rw, http.StatusNotFound, codersdk.Response{
			Message: "Anthropic integration is not configured for this organization.",
		})
		return
	}

	apiKey, err := s.apiKeyForUser(ctx, user.ID)
	if errors.Is(err, ErrMissingAPIKey) {
		writeMissingAPIKey(ctx, rw)
		return
	}
	if err != nil {
		s.Logger.Error(ctx, "load anthropic api key", slog.Error(err), slog.F("user_id", user.ID))
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error reading the user's Anthropic API key.",
			Detail:  err.Error(),
		})
		return
	}

	client := anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(apiKey)}, s.ClientOptions...)...)

	agents := make([]codersdk.AnthropicAgent, 0)
	iter := client.Beta.Agents.ListAutoPaging(ctx, anthropic.BetaAgentListParams{})
	for iter.Next() {
		a := iter.Current()
		agents = append(agents, codersdk.AnthropicAgent{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			Model:       a.Model.ID,
			Version:     a.Version,
			Archived:    !a.ArchivedAt.IsZero(),
			CreatedAt:   a.CreatedAt,
			UpdatedAt:   a.UpdatedAt,
		})
	}
	if err := iter.Err(); err != nil {
		s.Logger.Warn(ctx, "list anthropic agents", slog.Error(err), slog.F("user_id", user.ID))
		httpapi.Write(ctx, rw, http.StatusBadGateway, codersdk.Response{
			Message: "Anthropic rejected the agents list request.",
			Detail:  err.Error(),
		})
		return
	}

	httpapi.Write(ctx, rw, http.StatusOK, codersdk.AnthropicAgentsResponse{Agents: agents})
}

// writeMissingAPIKey produces the 412 response handlers return when
// the path user has no ANTHROPIC_API_KEY user secret. Centralized so
// every endpoint speaks the same wire shape and the UI can recognize
// the precondition on a single status code.
func writeMissingAPIKey(ctx context.Context, rw http.ResponseWriter) {
	httpapi.Write(ctx, rw, http.StatusPreconditionFailed, codersdk.Response{
		Message: "Anthropic API key is not configured for this user.",
		Detail:  "Add a user secret named " + codersdk.AnthropicAPIKeySecretName + " in Settings -> Anthropic.",
	})
}
