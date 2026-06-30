package sessions

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/codersdk"
)

// ErrMissingAPIKey is returned by [Service.apiKeyForUser] when the
// target user has not configured a Claude API key as a user secret.
// Handlers translate it into an HTTP 412 with guidance pointing the
// caller at the Settings -> Anthropic page.
var ErrMissingAPIKey = xerrors.New("user does not have an ANTHROPIC_API_KEY user secret configured")

// AnthropicAPIKeySecretName re-exports the well-known user-secret
// name from codersdk so handlers and tests in this package can
// reference a short local symbol.
const AnthropicAPIKeySecretName = codersdk.AnthropicAPIKeySecretName

// apiKeyForUser fetches the user's stored Anthropic API key from the
// user_secrets table. The returned value is the plaintext key suitable
// for passing to [option.WithAPIKey].
//
// Returns [ErrMissingAPIKey] when the secret does not exist so handlers
// can render a precondition-failed response.
func (s *Service) apiKeyForUser(ctx context.Context, userID uuid.UUID) (string, error) {
	secret, err := s.DB.GetUserSecretByUserIDAndName(ctx, database.GetUserSecretByUserIDAndNameParams{
		UserID: userID,
		Name:   codersdk.AnthropicAPIKeySecretName,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMissingAPIKey
	}
	if err != nil {
		return "", xerrors.Errorf("read user secret %q: %w", codersdk.AnthropicAPIKeySecretName, err)
	}
	if secret.Value == "" {
		return "", ErrMissingAPIKey
	}
	return secret.Value, nil
}
