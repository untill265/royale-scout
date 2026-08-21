package cardcache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"royalescout/internal/application/battlelog"
)

func TestDataURLDownloadsOnceAndUsesMemoryCache(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte("\x89PNG\r\n\x1a\nimage-data"))
	}))
	defer server.Close()

	cache, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cache.http = server.Client()
	request := battlelog.CardImageRequest{Key: "26000000", URL: server.URL + "/card.png"}
	first, err := cache.DataURL(context.Background(), request)
	if err != nil {
		t.Fatalf("DataURL() error = %v", err)
	}
	second, err := cache.DataURL(context.Background(), request)
	if err != nil {
		t.Fatalf("DataURL() error = %v", err)
	}
	if first != second || !strings.HasPrefix(first, "data:image/png;base64,") {
		t.Fatalf("data URLs differ or are invalid")
	}
	if requests.Load() != 1 {
		t.Fatalf("HTTP requests = %d, want 1", requests.Load())
	}
}

func TestSafeKeyAcceptsCardAndEvolutionKeysOnly(t *testing.T) {
	accepted := []string{"26000000", "26000087-evolution", "27000009-hero"}
	for _, key := range accepted {
		if !safeKey(key) {
			t.Fatalf("safeKey(%q) = false, want true", key)
		}
	}

	rejected := []string{"", "-evolution", "26000087-other", "../26000087", "26000087.png"}
	for _, key := range rejected {
		if safeKey(key) {
			t.Fatalf("safeKey(%q) = true, want false", key)
		}
	}
}
