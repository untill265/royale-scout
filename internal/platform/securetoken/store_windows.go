//go:build windows

package securetoken

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrNotFound = errors.New("secure API token is not stored")

var applicationEntropy = []byte("RoyaleScout/DPAPI/v1")

type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (store *Store) Load() (string, error) {
	encrypted, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read protected API token: %w", err)
	}
	if len(encrypted) == 0 {
		return "", errors.New("protected API token is empty")
	}

	plain, err := unprotect(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt API token for this Windows user: %w", err)
	}
	token := strings.TrimSpace(string(plain))
	clear(plain)
	if token == "" {
		return "", errors.New("decrypted API token is empty")
	}
	return token, nil
}

func (store *Store) Save(token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return errors.New("API token is empty")
	}
	encrypted, err := protect([]byte(trimmed))
	if err != nil {
		return fmt.Errorf("encrypt API token for this Windows user: %w", err)
	}
	defer clear(encrypted)

	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create protected token directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary protected token: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict temporary protected token: %w", err)
	}
	if _, err := temporary.Write(encrypted); err != nil {
		return fmt.Errorf("write temporary protected token: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush temporary protected token: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary protected token: %w", err)
	}
	if err := os.Rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("replace protected API token: %w", err)
	}
	committed = true
	return nil
}

func (store *Store) Delete() error {
	err := os.Remove(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete protected API token: %w", err)
	}
	return nil
}

func protect(plain []byte) ([]byte, error) {
	input := blob(plain)
	entropy := blob(applicationEntropy)
	var output windows.DataBlob
	description, err := windows.UTF16PtrFromString("Royale Scout API token")
	if err != nil {
		return nil, err
	}
	if err := windows.CryptProtectData(&input, description, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer freeBlob(&output)
	runtime.KeepAlive(plain)
	runtime.KeepAlive(applicationEntropy)
	return copyBlob(output), nil
}

func unprotect(encrypted []byte) ([]byte, error) {
	input := blob(encrypted)
	entropy := blob(applicationEntropy)
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer freeBlob(&output)
	runtime.KeepAlive(encrypted)
	runtime.KeepAlive(applicationEntropy)
	return copyBlob(output), nil
}

func blob(value []byte) windows.DataBlob {
	if len(value) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(value)), Data: &value[0]}
}

func copyBlob(value windows.DataBlob) []byte {
	if value.Size == 0 || value.Data == nil {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(value.Data, int(value.Size))...)
}

func freeBlob(value *windows.DataBlob) {
	if value.Data == nil {
		return
	}
	_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(value.Data))))
	value.Data = nil
	value.Size = 0
}
