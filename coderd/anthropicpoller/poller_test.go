package anthropicpoller_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"cdr.dev/slog/v3/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/anthropicpoller"
	"github.com/coder/coder/v2/testutil"
)

func TestNew_RejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  anthropicpoller.OrgConfig
	}{
		{
			name: "missing org id",
			cfg: anthropicpoller.OrgConfig{
				EnvironmentID:  "env_test",
				EnvironmentKey: "sk-ant-oat01-test",
			},
		},
		{
			name: "missing environment id",
			cfg: anthropicpoller.OrgConfig{
				OrgID:          uuid.New(),
				EnvironmentKey: "sk-ant-oat01-test",
			},
		},
		{
			name: "missing environment key",
			cfg: anthropicpoller.OrgConfig{
				OrgID:         uuid.New(),
				EnvironmentID: "env_test",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := anthropicpoller.New(
				tc.cfg,
				anthropic.NewClient(),
				&anthropicpoller.NoopDispatcher{},
				slogtest.Make(t, nil),
			)
			require.Error(t, err)
		})
	}
}

func TestNew_RejectsNilDispatcher(t *testing.T) {
	t.Parallel()

	_, err := anthropicpoller.New(
		validConfig(t),
		anthropic.NewClient(),
		nil,
		slogtest.Make(t, nil),
	)
	require.ErrorContains(t, err, "dispatcher")
}

func TestNew_Succeeds(t *testing.T) {
	t.Parallel()

	p, err := anthropicpoller.New(
		validConfig(t),
		anthropic.NewClient(),
		&anthropicpoller.NoopDispatcher{},
		slogtest.Make(t, nil),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
}

// TestPoller_Run_StopsOnContextCancel verifies the poller exits cleanly
// when its context is canceled. The mock Anthropic server returns the
// SDK's "empty queue" sentinel (200 with a JSON null body) for every
// request, so the poller idles and we have a deterministic teardown path.
func TestPoller_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("null"))
	}))
	t.Cleanup(mock.Close)

	client := anthropic.NewClient(option.WithBaseURL(mock.URL))

	p, err := anthropicpoller.New(
		validConfig(t),
		client,
		&anthropicpoller.NoopDispatcher{},
		slogtest.Make(t, &slogtest.Options{IgnoreErrors: true}),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx)
	}()

	cancel()

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-testutil.Context(t, testutil.WaitLong).Done():
		t.Fatal("poller did not stop after context cancel")
	}
}

func validConfig(t *testing.T) anthropicpoller.OrgConfig {
	t.Helper()
	return anthropicpoller.OrgConfig{
		OrgID:          uuid.New(),
		EnvironmentID:  "env_test",
		EnvironmentKey: "sk-ant-oat01-test",
	}
}
