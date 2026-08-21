//go:build !windows

package securetoken

import "errors"

var (
	ErrNotFound    = errors.New("secure API token is not stored")
	errUnsupported = errors.New("secure token storage is supported only on Windows")
)

type Store struct{}

func New(string) *Store              { return &Store{} }
func (*Store) Load() (string, error) { return "", errUnsupported }
func (*Store) Save(string) error     { return errUnsupported }
func (*Store) Delete() error         { return errUnsupported }
