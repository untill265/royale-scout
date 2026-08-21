package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	tokenEnvironmentVariable = "CLASH_ROYALE_API_TOKEN"
	localConfigFilename      = "config.local.json"
	placeholderToken         = "PASTE_TOKEN_HERE"
)

var ErrTokenMissing = errors.New("Clash Royale API token is not configured")

type Config struct {
	ClashRoyaleAPIToken string `json:"clashRoyaleApiToken"`
}

func Load() (Config, error) {
	if token := strings.TrimSpace(os.Getenv(tokenEnvironmentVariable)); isUsableToken(token) {
		return Config{ClashRoyaleAPIToken: token}, nil
	}

	paths, err := candidatePaths()
	if err != nil {
		return Config{}, err
	}

	for _, path := range paths {
		loaded, found, loadErr := loadFile(path)
		if loadErr != nil {
			return Config{}, loadErr
		}
		if found && isUsableToken(loaded.ClashRoyaleAPIToken) {
			loaded.ClashRoyaleAPIToken = strings.TrimSpace(loaded.ClashRoyaleAPIToken)
			return loaded, nil
		}
	}

	return Config{}, ErrTokenMissing
}

func candidatePaths() ([]string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	executable, _ := os.Executable()
	return candidatePathsAt(workingDirectory, executable), nil
}

func candidatePathsAt(workingDirectory, executable string) []string {
	paths := []string{filepath.Join(workingDirectory, localConfigFilename)}
	if executable != "" {
		executableDirectory := filepath.Dir(executable)
		executableConfig := filepath.Join(executableDirectory, localConfigFilename)
		if executableConfig != paths[0] {
			paths = append(paths, executableConfig)
		}

		// A local Wails build lives in build/bin. Keep the developer token in the
		// project root so the generated executable can be opened directly without
		// copying a secret into the build output.
		if strings.EqualFold(filepath.Base(executableDirectory), "bin") &&
			strings.EqualFold(filepath.Base(filepath.Dir(executableDirectory)), "build") {
			projectConfig := filepath.Join(executableDirectory, "..", "..", localConfigFilename)
			paths = append(paths, filepath.Clean(projectConfig))
		}
	}
	return paths
}

func loadFile(path string) (Config, bool, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read local config %q: %w", path, err)
	}

	var loaded Config
	if err := json.Unmarshal(contents, &loaded); err != nil {
		return Config{}, true, fmt.Errorf("parse local config %q: %w", path, err)
	}

	return loaded, true, nil
}

func isUsableToken(token string) bool {
	trimmed := strings.TrimSpace(token)
	return trimmed != "" && trimmed != placeholderToken
}
