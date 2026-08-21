package netpolicy

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRestrictedTransportAllowsOnlyConfiguredHTTPSHost(t *testing.T) {
	called := false
	transport := &restrictedTransport{
		allowedHosts: map[string]struct{}{"api.clashroyale.com": {}},
		base: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	}

	allowed, _ := http.NewRequest(http.MethodGet, "https://api.clashroyale.com/v1/cards", nil)
	if _, err := transport.RoundTrip(allowed); err != nil {
		t.Fatalf("official API request error = %v", err)
	}
	if !called {
		t.Fatal("official API request did not reach the base transport")
	}

	for _, rawURL := range []string{
		"http://api.clashroyale.com/v1/cards",
		"https://api.clashroyale.com.evil.test/v1/cards",
		"https://example.test/v1/cards",
		"https://api.clashroyale.com:8443/v1/cards",
	} {
		request, _ := http.NewRequest(http.MethodGet, rawURL, nil)
		if _, err := transport.RoundTrip(request); err == nil {
			t.Fatalf("RoundTrip(%q) succeeded, want policy error", rawURL)
		}
	}
}
