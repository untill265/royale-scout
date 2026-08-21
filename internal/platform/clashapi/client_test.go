package clashapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocationsSendsBearerTokenAndDecodesPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.URL.Query().Get("limit") != "1000" {
			t.Fatalf("limit = %q", request.URL.Query().Get("limit"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"items":[{"id":57000007,"name":"Afghanistan","isCountry":true,"countryCode":"AF"}],"paging":{"cursors":{}}}`))
	}))
	defer server.Close()

	client, err := New("secret-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	page, meta, err := client.Locations(context.Background(), 1000, "")
	if err != nil {
		t.Fatalf("Locations() error = %v", err)
	}
	if meta.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", meta.StatusCode)
	}
	if len(page.Items) != 1 || page.Items[0].CountryCode != "AF" {
		t.Fatalf("locations = %#v", page.Items)
	}
}

func TestBattleLogEscapesPlayerTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.EscapedPath() != "/players/%23ABC123/battlelog" {
			t.Fatalf("path = %q", request.URL.EscapedPath())
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := New("secret-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, _, err := client.BattleLog(context.Background(), "#ABC123"); err != nil {
		t.Fatalf("BattleLog() error = %v", err)
	}
}

func TestAPIErrorDoesNotContainToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"reason":"accessDenied","message":"bad IP"}`))
	}))
	defer server.Close()

	client, err := New("secret-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithRetry(1, 0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, _, err = client.Locations(context.Background(), 1, "")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %v, want APIError", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestRetries429ThenSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		writer.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"reason":"rateLimit"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"items":[],"paging":{"cursors":{}}}`))
	}))
	defer server.Close()

	client, err := New(
		"secret-token",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetry(2, time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, meta, err := client.Locations(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("Locations() error = %v", err)
	}
	if meta.Attempts != 2 || attempts != 2 {
		t.Fatalf("attempts = meta %d, server %d", meta.Attempts, attempts)
	}
}
