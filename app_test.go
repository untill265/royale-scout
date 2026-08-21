package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"royalescout/internal/platform/clashapi"
)

func TestFirstRunRequiresSetupAndDeletesOnlyManagedData(t *testing.T) {
	dataDirectory := t.TempDir()
	t.Setenv("ROYALE_SCOUT_DATA_DIR", dataDirectory)
	t.Setenv(ignoreLegacyConfigEnv, "1")

	app := NewApp()
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })

	status := app.GetStatus()
	if status.State != "setupRequired" || status.HasToken {
		t.Fatalf("GetStatus() = %+v, want setupRequired without token", status)
	}

	unmanagedPath := filepath.Join(dataDirectory, "keep-user-file.txt")
	if err := os.WriteFile(unmanagedPath, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if result := app.DeleteAllLocalData(); !result.Success {
		t.Fatalf("DeleteAllLocalData() = %+v, want success", result)
	}
	if _, err := os.Stat(unmanagedPath); err != nil {
		t.Fatalf("DeleteAllLocalData removed unmanaged file: %v", err)
	}
	if got := app.GetStatus().State; got != "setupRequired" {
		t.Fatalf("GetStatus().State = %q after delete, want setupRequired", got)
	}
}

func TestClassifyTokenError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "unauthorised", err: &clashapi.APIError{StatusCode: http.StatusUnauthorized}, want: "invalid"},
		{name: "forbidden IP", err: &clashapi.APIError{StatusCode: http.StatusForbidden, Message: "Invalid IP"}, want: "ipNotAllowed"},
		{name: "other forbidden", err: &clashapi.APIError{StatusCode: http.StatusForbidden}, want: "invalid"},
		{name: "rate limited", err: &clashapi.APIError{StatusCode: http.StatusTooManyRequests}, want: "rateLimited"},
		{name: "server unavailable", err: &clashapi.APIError{StatusCode: http.StatusBadGateway}, want: "serviceUnavailable"},
		{name: "timeout", err: context.DeadlineExceeded, want: "network"},
		{name: "unknown", err: errors.New("unexpected"), want: "verificationFailed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyTokenError(test.err); got != test.want {
				t.Fatalf("classifyTokenError() = %q, want %q", got, test.want)
			}
		})
	}
}
