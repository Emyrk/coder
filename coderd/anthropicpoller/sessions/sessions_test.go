package sessions_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cdr.dev/slog/v3"
	"cdr.dev/slog/v3/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/anthropicpoller"
	"github.com/coder/coder/v2/coderd/anthropicpoller/sessions"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbgen"
	"github.com/coder/coder/v2/coderd/database/dbtestutil"
	"github.com/coder/coder/v2/testutil"
)

func validConfig(t *testing.T) anthropicpoller.OrgConfig {
	t.Helper()
	return anthropicpoller.OrgConfig{
		OrgID:          uuid.New(),
		EnvironmentID:  "env_test",
		EnvironmentKey: "sk-ant-oat01-test",
	}
}

func TestNew_RejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	db, _ := dbtestutil.NewDB(t)
	_, err := sessions.New(anthropicpoller.OrgConfig{}, db, slog.Logger{})
	require.Error(t, err, "expected config validation to fail")
}

func TestNew_RequiresDB(t *testing.T) {
	t.Parallel()

	_, err := sessions.New(validConfig(t), nil, slog.Logger{})
	require.Error(t, err, "expected nil db to fail")
}

func TestNew_Succeeds(t *testing.T) {
	t.Parallel()

	db, _ := dbtestutil.NewDB(t)
	svc, err := sessions.New(validConfig(t), db, slogtest.Make(t, nil))
	require.NoError(t, err)
	require.NotNil(t, svc)
}

func TestAPIKeyForUser_ReturnsValueWhenSecretExists(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t, testutil.WaitShort)

	db, _ := dbtestutil.NewDB(t)
	user := dbgen.User(t, db, database.User{})
	const keyValue = "sk-ant-api03-fixture"
	dbgen.UserSecret(t, db, database.UserSecret{
		UserID: user.ID,
		Name:   sessions.AnthropicAPIKeySecretName,
		Value:  keyValue,
	})

	got, err := apiKeyForUserViaExportedHandler(ctx, t, db, user.ID)
	require.NoError(t, err)
	assert.Equal(t, keyValue, got)
}

func TestAPIKeyForUser_ReturnsMissingErrorWhenAbsent(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t, testutil.WaitShort)

	db, _ := dbtestutil.NewDB(t)
	user := dbgen.User(t, db, database.User{})

	_, err := apiKeyForUserViaExportedHandler(ctx, t, db, user.ID)
	require.ErrorIs(t, err, sessions.ErrMissingAPIKey)
}

func TestStampReservedMetadata_AlwaysOverwritesCoderUserID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		in          map[string]string
		coderUserID string
	}{
		{
			name:        "nil input is safe",
			in:          nil,
			coderUserID: "u-1",
		},
		{
			name:        "caller value is overwritten",
			in:          map[string]string{"coder_user_id": "impostor", "feature": "x"},
			coderUserID: "u-real",
		},
		{
			name:        "unrelated keys pass through",
			in:          map[string]string{"team": "core", "trace": "abc"},
			coderUserID: "u-2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sessions.ExportedStampReservedMetadata(tc.in, tc.coderUserID)
			assert.Equal(t, tc.coderUserID, got["coder_user_id"], "coder_user_id must be stamped")
			for k, v := range tc.in {
				if k == "coder_user_id" {
					continue
				}
				assert.Equal(t, v, got[k], "unrelated key %q lost", k)
			}
		})
	}
}

// apiKeyForUserViaExportedHandler exercises the unexported
// (*Service).apiKeyForUser through a thin shim built on top of the
// package's exported surface. Keeping the test goal narrow lets us
// verify the secret-lookup path before any HTTP wiring lands.
func apiKeyForUserViaExportedHandler(ctx context.Context, t *testing.T, db database.Store, userID uuid.UUID) (string, error) {
	t.Helper()
	svc, err := sessions.New(validConfig(t), db, slogtest.Make(t, nil))
	require.NoError(t, err)
	return sessions.ExportedAPIKeyForUser(ctx, svc, userID)
}
