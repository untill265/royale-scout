//go:build windows

package securetoken

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreEncryptsLoadsAndDeletesToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.bin")
	store := New(path)

	if _, err := store.Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() before save error = %v, want %v", err, ErrNotFound)
	}
	if err := store.Save("  secret-token  "); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded != "secret-token" {
		t.Fatalf("Load() = %q, want secret-token", loaded)
	}
	if err := store.Save("replacement-token"); err != nil {
		t.Fatalf("Save(replacement) error = %v", err)
	}
	loaded, err = store.Load()
	if err != nil {
		t.Fatalf("Load(replacement) error = %v", err)
	}
	if loaded != "replacement-token" {
		t.Fatalf("Load(replacement) = %q, want replacement-token", loaded)
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() after delete error = %v, want %v", err, ErrNotFound)
	}
}

func TestStoredBytesDoNotContainPlainToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.bin")
	store := New(path)
	const token = "unique-plain-token-value"
	if err := store.Save(token); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(contents, []byte(token)) {
		t.Fatal("protected credential file contains the plain API token")
	}
}
