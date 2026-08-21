package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadUsesEnvironmentToken(t *testing.T) {
	t.Setenv(tokenEnvironmentVariable, " test-token ")

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.ClashRoyaleAPIToken != "test-token" {
		t.Fatalf("Load() token = %q, want %q", loaded.ClashRoyaleAPIToken, "test-token")
	}
}

func TestLoadRejectsPlaceholder(t *testing.T) {
	t.Setenv(tokenEnvironmentVariable, placeholderToken)

	temporaryDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(temporaryDirectory); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })

	_, err = Load()
	if !errors.Is(err, ErrTokenMissing) {
		t.Fatalf("Load() error = %v, want %v", err, ErrTokenMissing)
	}
}

func TestCandidatePathsIncludeProjectRootForLocalBuild(t *testing.T) {
	project := t.TempDir()
	workingDirectory := filepath.Join(project, "somewhere-else")
	executable := filepath.Join(project, "build", "bin", "royale-scout.exe")

	got := candidatePathsAt(workingDirectory, executable)
	want := []string{
		filepath.Join(workingDirectory, localConfigFilename),
		filepath.Join(project, "build", "bin", localConfigFilename),
		filepath.Join(project, localConfigFilename),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidatePathsAt() = %#v, want %#v", got, want)
	}
}
