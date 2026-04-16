package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if had {
			err = os.Setenv(key, old)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("failed to restore %s: %v", key, err)
		}
	})
}

func TestLoadDotEnvFromLoadsMissingVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("HN_TRANSLATE_API_KEY=test-key\nHN_TRANSLATE_LANG=Japanese\n"), 0o600); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	unsetEnvForTest(t, "HN_TRANSLATE_API_KEY")
	unsetEnvForTest(t, "HN_TRANSLATE_LANG")

	if err := loadDotEnvFrom(dir); err != nil {
		t.Fatalf("expected .env to load: %v", err)
	}

	if got := os.Getenv("HN_TRANSLATE_API_KEY"); got != "test-key" {
		t.Fatalf("expected HN_TRANSLATE_API_KEY from .env, got %q", got)
	}
	if got := os.Getenv("HN_TRANSLATE_LANG"); got != "Japanese" {
		t.Fatalf("expected HN_TRANSLATE_LANG from .env, got %q", got)
	}
}

func TestLoadDotEnvFromDoesNotOverrideExistingVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("HN_TRANSLATE_API_KEY=from-file\n"), 0o600); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	t.Setenv("HN_TRANSLATE_API_KEY", "from-env")

	if err := loadDotEnvFrom(dir); err != nil {
		t.Fatalf("expected .env to load: %v", err)
	}

	if got := os.Getenv("HN_TRANSLATE_API_KEY"); got != "from-env" {
		t.Fatalf("expected existing env var to win, got %q", got)
	}
}

func TestLoadDotEnvFromIgnoresMissingFile(t *testing.T) {
	if err := loadDotEnvFrom(t.TempDir()); err != nil {
		t.Fatalf("expected missing .env to be ignored: %v", err)
	}
}
