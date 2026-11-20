package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SPROG666/go-autumn-2026/internal/config"
)

func example(t *testing.T) {
	t.Helper()
	source, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(source), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatal("invalid env example")
		}
		t.Setenv(key, value)
	}
}

func TestPositionDefaults(t *testing.T) {
	example(t)
	for _, key := range []string{"POSITION_CHECK_INTERVAL", "POSITION_STALE_AFTER", "POSITION_WORKERS", "POSITION_QUEUE_SIZE", "PUSH_REQUEST_TIMEOUT"} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PositionCheckInterval != 30*time.Second || cfg.PositionStaleAfter != time.Minute || cfg.PositionWorkers != 4 || cfg.PositionQueueSize != 100 || cfg.PushRequestTimeout != time.Second {
		t.Fatalf("incorrect polling defaults")
	}
}

func TestInvalidPositionConfiguration(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{"POSITION_CHECK_INTERVAL", "0s"},
		{"POSITION_STALE_AFTER", "-1s"},
		{"POSITION_WORKERS", "0"},
		{"POSITION_QUEUE_SIZE", "0"},
		{"PUSH_REQUEST_TIMEOUT", "0s"},
		{"PUSH_HTTP_URL", ""},
		{"PUSH_HTTP_URL", "invalid"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			example(t)
			t.Setenv(test.key, test.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("invalid configuration accepted", test.key)
			}
		})
	}
}
