package config

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateConfigEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv("HN_TRANSLATE_API_URL", "")
	t.Setenv("HN_TRANSLATE_API_KEY", "")
	t.Setenv("HN_TRANSLATE_MODEL", "")
	t.Setenv("HN_TRANSLATE_LANG", "")
	t.Setenv("OPENAI_API_KEY", "")
	return root
}

func writeCodexAuth(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "codex")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTranslateConfigUsesHNAPIKeyFirst(t *testing.T) {
	root := isolateConfigEnv(t)
	if err := SaveConfig(Config{Translate: &TranslateConfig{APIKey: "sk-config"}}); err != nil {
		t.Fatal(err)
	}
	writeCodexAuth(t, root, `{"OPENAI_API_KEY":"sk-codex"}`)
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("HN_TRANSLATE_API_KEY", "sk-hn")

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "sk-hn" {
		t.Fatalf("expected HN_TRANSLATE_API_KEY to win, got %q", cfg.APIKey)
	}
}

func TestLoadTranslateConfigKeepsConfiguredAPIKey(t *testing.T) {
	isolateConfigEnv(t)
	if err := SaveConfig(Config{Translate: &TranslateConfig{APIKey: "sk-config"}}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "sk-openai")

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "sk-config" {
		t.Fatalf("expected config API key to win over generic OPENAI_API_KEY, got %q", cfg.APIKey)
	}
}

func TestLoadTranslateConfigUsesOpenAIAPIKeyFallback(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai")

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "sk-openai" {
		t.Fatalf("expected OPENAI_API_KEY fallback, got %q", cfg.APIKey)
	}
}

func TestLoadTranslateConfigUsesCodexAPIKeyFallback(t *testing.T) {
	root := isolateConfigEnv(t)
	writeCodexAuth(t, root, `{"auth_mode":"api","OPENAI_API_KEY":" sk-codex "}`)

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "sk-codex" {
		t.Fatalf("expected Codex OPENAI_API_KEY fallback, got %q", cfg.APIKey)
	}
}

func TestLoadTranslateConfigIgnoresCodexChatGPTTokens(t *testing.T) {
	root := isolateConfigEnv(t)
	writeCodexAuth(t, root, `{
		"auth_mode":"chatgpt",
		"OPENAI_API_KEY":null,
		"tokens":{"access_token":"not-an-api-key","refresh_token":"not-an-api-key"}
	}`)

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "" {
		t.Fatalf("expected ChatGPT OAuth tokens to be ignored, got %q", cfg.APIKey)
	}
}
