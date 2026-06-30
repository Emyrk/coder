package sessions

import (
	"errors"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"cdr.dev/slog/v3"
	"github.com/coder/coder/v2/coderd/httpapi"
	"github.com/coder/coder/v2/coderd/httpmw"
	"github.com/coder/coder/v2/codersdk"
)

// CreateSession creates an Anthropic session for the path user via
// the Anthropic platform API. The request shape is:
//
//	POST /api/v2/anthropic/{organization_id}/sessions/{userid}
//
// The body is [codersdk.CreateAnthropicSessionRequest]. Coderd binds
// the session to the org's configured environment (so the inner
// poller will eventually claim its work) and stamps reserved metadata
// keys, overwriting any values the client supplied for those keys.
//
// @Summary Create an Anthropic session for the user
// @ID create-anthropic-session
// @Security CoderSessionToken
// @Accept json
// @Produce json
// @Tags Anthropic
// @Param organization path string true "Organization ID"
// @Param user path string true "User ID, username, or me"
// @Param request body codersdk.CreateAnthropicSessionRequest true "Create session request"
// @Success 201 {object} codersdk.AnthropicSession
// @Router /api/v2/anthropic/{organization}/sessions/{user} [post]
func (s *Service) CreateSession(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	org := httpmw.OrganizationParam(r)
	user := httpmw.UserParam(r)

	if org.ID != s.Config.OrgID {
		httpapi.Write(ctx, rw, http.StatusNotFound, codersdk.Response{
			Message: "Anthropic integration is not configured for this organization.",
		})
		return
	}

	var req codersdk.CreateAnthropicSessionRequest
	if !httpapi.Read(ctx, rw, r, &req) {
		return
	}
	if req.AgentID == "" {
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "agent_id is required.",
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

	metadata := stampReservedMetadata(req.Metadata, user.ID.String())

	client := anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(apiKey)}, s.ClientOptions...)...)
	params := anthropic.BetaSessionNewParams{
		Agent: anthropic.BetaSessionNewParamsAgentUnion{
			OfString: param.NewOpt(req.AgentID),
		},
		EnvironmentID: s.Config.EnvironmentID,
		Metadata:      metadata,
	}
	if req.Title != "" {
		params.Title = param.NewOpt(req.Title)
	}

	created, err := client.Beta.Sessions.New(ctx, params)
	if err != nil {
		s.Logger.Warn(ctx, "create anthropic session",
			slog.Error(err),
			slog.F("user_id", user.ID),
			slog.F("agent_id", req.AgentID),
			slog.F("environment_id", s.Config.EnvironmentID),
		)
		httpapi.Write(ctx, rw, http.StatusBadGateway, codersdk.Response{
			Message: "Anthropic rejected the session create request.",
			Detail:  err.Error(),
		})
		return
	}

	s.Logger.Info(ctx, "anthropic session created",
		slog.F("session_id", created.ID),
		slog.F("user_id", user.ID),
		slog.F("agent_id", req.AgentID),
		slog.F("environment_id", s.Config.EnvironmentID),
	)

	httpapi.Write(ctx, rw, http.StatusCreated, codersdk.AnthropicSession{
		ID:            created.ID,
		AgentID:       req.AgentID,
		EnvironmentID: s.Config.EnvironmentID,
		Title:         created.Title,
		Metadata:      metadata,
		CreatedAt:     created.CreatedAt,
		CoderUserID:   user.ID,
	})
}

// stampReservedMetadata clones the caller's metadata map and writes
// the Coder reserved keys on top. A nil input is allowed; the
// returned map is always non-nil and safe to forward to the SDK.
//
// Today only coder_user_id is stamped. The other entries in
// [codersdk.AnthropicReservedMetadataKeys] are reserved up-front so
// silent overwrites stay correct when those stamps are added.
func stampReservedMetadata(in map[string]string, coderUserID string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	out["coder_user_id"] = coderUserID
	return out
}
