package sessions

import (
	"errors"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/go-chi/chi/v5"

	"cdr.dev/slog/v3"
	"github.com/coder/coder/v2/coderd/httpapi"
	"github.com/coder/coder/v2/coderd/httpmw"
	"github.com/coder/coder/v2/codersdk"
)

// SendEvent forwards a user-message event to an Anthropic session.
// The request shape is:
//
//	POST /api/v2/anthropic/{organization}/sessions/{user}/{session}/events
//
// The body is [codersdk.SendAnthropicEventRequest]. Coderd validates
// the path organization against the configured Anthropic org, fetches
// the user's Anthropic API key from user_secrets, and forwards the
// event to Anthropic via the SDK. The session must already exist on
// Anthropic; coderd does not validate session ownership today, but
// Anthropic will reject sessions the caller's API key cannot access.
//
// @Summary Send an event to an Anthropic session
// @ID send-anthropic-event
// @Security CoderSessionToken
// @Accept json
// @Produce json
// @Tags Anthropic
// @Param organization path string true "Organization ID"
// @Param user path string true "User ID, username, or me"
// @Param session path string true "Anthropic session ID"
// @Param request body codersdk.SendAnthropicEventRequest true "Send event request"
// @Success 200 {object} codersdk.SendAnthropicEventResponse
// @Router /api/v2/anthropic/{organization}/sessions/{user}/{session}/events [post]
func (s *Service) SendEvent(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	org := httpmw.OrganizationParam(r)
	user := httpmw.UserParam(r)
	sessionID := chi.URLParam(r, "session")

	if org.ID != s.Config.OrgID {
		httpapi.Write(ctx, rw, http.StatusNotFound, codersdk.Response{
			Message: "Anthropic integration is not configured for this organization.",
		})
		return
	}
	if sessionID == "" {
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "session path parameter is required.",
		})
		return
	}

	var req codersdk.SendAnthropicEventRequest
	if !httpapi.Read(ctx, rw, r, &req) {
		return
	}
	if req.Text == "" {
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "text is required.",
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
	params := anthropic.BetaSessionEventSendParams{
		Events: []anthropic.BetaManagedAgentsEventParamsUnion{
			{
				OfUserMessage: &anthropic.BetaManagedAgentsUserMessageEventParams{
					Type: anthropic.BetaManagedAgentsUserMessageEventParamsTypeUserMessage,
					Content: []anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{
						{
							OfText: &anthropic.BetaManagedAgentsTextBlockParam{
								Text: req.Text,
								Type: anthropic.BetaManagedAgentsTextBlockTypeText,
							},
						},
					},
				},
			},
		},
	}

	sent, err := client.Beta.Sessions.Events.Send(ctx, sessionID, params)
	if err != nil {
		s.Logger.Warn(ctx, "send anthropic session event",
			slog.Error(err),
			slog.F("user_id", user.ID),
			slog.F("session_id", sessionID),
		)
		httpapi.Write(ctx, rw, http.StatusBadGateway, codersdk.Response{
			Message: "Anthropic rejected the send-event request.",
			Detail:  err.Error(),
		})
		return
	}

	events := make([]codersdk.AnthropicEvent, 0, len(sent.Data))
	for _, e := range sent.Data {
		events = append(events, codersdk.AnthropicEvent{
			ID:          e.ID,
			Type:        e.Type,
			ProcessedAt: e.ProcessedAt,
		})
	}

	s.Logger.Info(ctx, "anthropic session event sent",
		slog.F("session_id", sessionID),
		slog.F("user_id", user.ID),
		slog.F("event_count", len(events)),
	)

	httpapi.Write(ctx, rw, http.StatusOK, codersdk.SendAnthropicEventResponse{
		Events: events,
	})
}
