package sessions

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/database"
)

// AnthropicAPIKeySecretName is the well-known [database.UserSecret]
// name that holds a Coder user's Anthropic platform API key. Users
// create it via the Settings -> Secrets page; the value is sent on
// outbound requests to Anthropic on their behalf.
//
// The name matches the SDK's [ANTHROPIC_API_KEY] env var so the same
// secret can also be injected into workspaces by the secret machinery
// later without renaming.
//
// [ANTHROPIC_API_KEY]: https://github.com/anthropics/anthropic-sdk-go#with-environment-variables
//
//nolint:gosec // Constant holds the *name* of a secret, not its value.
const AnthropicAPIKeySecretName = "ANTHROPIC_API_KEY"

// ErrMissingAPIKey is returned by [Service.apiKeyForUser] when the
// target user has not configured a Claude API key as a user secret.
// Handlers translate it into an HTTP 412 with guidance pointing the
// caller at the Settings -> Secrets page.
var ErrMissingAPIKey = xerrors.New("user does not have an ANTHROPIC_API_KEY user secret configured")

// apiKeyForUser fetches the user's stored Anthropic API key from the
// user_secrets table. The returned value is the plaintext key suitable
// for passing to [option.WithAPIKey].
//
// Returns [ErrMissingAPIKey] when the secret does not exist so handlers
// can render a precondition-failed response.
func (s *Service) apiKeyForUser(ctx context.Context, userID uuid.UUID) (string, error) {
	secret, err := s.DB.GetUserSecretByUserIDAndName(ctx, database.GetUserSecretByUserIDAndNameParams{
		UserID: userID,
		Name:   AnthropicAPIKeySecretName,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMissingAPIKey
	}
	if err != nil {
		return "", xerrors.Errorf("read user secret %q: %w", AnthropicAPIKeySecretName, err)
	}
	if secret.Value == "" {
		return "", ErrMissingAPIKey
	}
	return secret.Value, nil
}
