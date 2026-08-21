package appdata

import (
	"fmt"
	"os"
	"path/filepath"
)

const dataDirectoryEnvironmentVariable = "ROYALE_SCOUT_DATA_DIR"

type Paths struct {
	Root        string
	Database    string
	CardCache   string
	Credentials string
}

func Resolve() (Paths, error) {
	root := os.Getenv(dataDirectoryEnvironmentVariable)
	if root == "" {
		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve user cache directory: %w", err)
		}
		root = filepath.Join(cacheRoot, "RoyaleScout")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve Royale Scout data directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Paths{}, fmt.Errorf("create Royale Scout data directory: %w", err)
	}
	return Paths{
		Root:        root,
		Database:    filepath.Join(root, "royale-scout.db"),
		CardCache:   filepath.Join(root, "cards"),
		Credentials: filepath.Join(root, "credentials.bin"),
	}, nil
}
