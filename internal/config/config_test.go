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
	t.Setenv("HN_TRANSLATE_REASONING_EFFORT", "")
	t.Setenv("HN_TRANSLATE_SERVICE_TIER", "")
	t.Setenv("HN_TRANSLATE_WIRE_API", "")
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
	if cfg.AuthSource != translateAuthSourceCodexAPIKey {
		t.Fatalf("expected Codex API key auth source, got %q", cfg.AuthSource)
	}
}

func TestLoadTranslateConfigUsesCodexChatGPTAccessToken(t *testing.T) {
	root := isolateConfigEnv(t)
	writeCodexAuth(t, root, `{
		"auth_mode":"chatgpt",
		"OPENAI_API_KEY":null,
		"tokens":{"access_token":" oauth-access-token ","refresh_token":"refresh-token","account_id":" account-123 "}
	}`)

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "oauth-access-token" {
		t.Fatalf("expected ChatGPT OAuth access token fallback, got %q", cfg.APIKey)
	}
	if cfg.AuthSource != translateAuthSourceCodexOAuth {
		t.Fatalf("expected Codex OAuth auth source, got %q", cfg.AuthSource)
	}
	if cfg.APIURL != defaultCodexOAuthAPIURL {
		t.Fatalf("expected Codex OAuth API URL %q, got %q", defaultCodexOAuthAPIURL, cfg.APIURL)
	}
	if cfg.WireAPI != translateWireAPICodexResponses {
		t.Fatalf("expected Codex OAuth wire API %q, got %q", translateWireAPICodexResponses, cfg.WireAPI)
	}
	if cfg.ChatGPTAccountID != "account-123" {
		t.Fatalf("expected ChatGPT account id, got %q", cfg.ChatGPTAccountID)
	}
	if cfg.Model != defaultCodexOAuthTranslateModel {
		t.Fatalf("expected Codex OAuth default model %q, got %q", defaultCodexOAuthTranslateModel, cfg.Model)
	}
	if cfg.ReasoningEffort != defaultCodexOAuthReasoningEffort {
		t.Fatalf("expected Codex OAuth reasoning effort %q, got %q", defaultCodexOAuthReasoningEffort, cfg.ReasoningEffort)
	}
	if cfg.ServiceTier != defaultCodexOAuthServiceTier {
		t.Fatalf("expected Codex OAuth service tier %q, got %q", defaultCodexOAuthServiceTier, cfg.ServiceTier)
	}
	if cfg.SetupHint != "" {
		t.Fatalf("expected no setup hint when OAuth token is usable, got %q", cfg.SetupHint)
	}
}

func TestLoadTranslateConfigNormalizesCodexSparkReasoning(t *testing.T) {
	root := isolateConfigEnv(t)
	writeCodexAuth(t, root, `{
		"auth_mode":"chatgpt",
		"OPENAI_API_KEY":null,
		"tokens":{"access_token":"oauth-access-token"}
	}`)
	t.Setenv("HN_TRANSLATE_MODEL", codexSparkTranslateModel)
	t.Setenv("HN_TRANSLATE_REASONING_EFFORT", "none")

	cfg := LoadTranslateConfig()

	if cfg.Model != codexSparkTranslateModel {
		t.Fatalf("expected Spark model, got %q", cfg.Model)
	}
	if cfg.ReasoningEffort != "low" {
		t.Fatalf("expected Spark reasoning effort to normalize to low, got %q", cfg.ReasoningEffort)
	}
}

func TestLoadTranslateConfigHintsWhenCodexChatGPTTokenMissing(t *testing.T) {
	root := isolateConfigEnv(t)
	writeCodexAuth(t, root, `{
		"auth_mode":"chatgpt",
		"OPENAI_API_KEY":null,
		"tokens":{"refresh_token":"refresh-token"}
	}`)

	cfg := LoadTranslateConfig()

	if cfg.APIKey != "" {
		t.Fatalf("expected missing ChatGPT access token to leave API key empty, got %q", cfg.APIKey)
	}
	if cfg.SetupHint == "" {
		t.Fatal("expected setup hint for missing ChatGPT access token")
	}
}
