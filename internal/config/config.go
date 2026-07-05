package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds global CLI configuration.
type Config struct {
	Theme     string           `json:"theme,omitempty"`
	Translate *TranslateConfig `json:"translate,omitempty"`
}

// TranslateConfig holds OpenAI-compatible translation settings.
type TranslateConfig struct {
	APIURL   string `json:"api_url,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
}

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "hn")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(configDir(), "config.json")
}

// LoadConfig loads the config from disk.
func LoadConfig() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// SaveConfig writes the config to disk.
func SaveConfig(cfg Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0o644)
}

// LoadTranslateConfig loads translation settings with environment overrides.
func LoadTranslateConfig() TranslateConfig {
	cfg, _ := LoadConfig()
	var tc TranslateConfig
	if cfg.Translate != nil {
		tc = *cfg.Translate
	}

	if v := os.Getenv("HN_TRANSLATE_API_URL"); v != "" {
		tc.APIURL = v
	}
	if v := os.Getenv("HN_TRANSLATE_API_KEY"); v != "" {
		tc.APIKey = v
	}
	if v := os.Getenv("HN_TRANSLATE_MODEL"); v != "" {
		tc.Model = v
	}
	if v := os.Getenv("HN_TRANSLATE_LANG"); v != "" {
		tc.Language = v
	}

	if tc.APIURL == "" {
		tc.APIURL = "https://api.openai.com/v1"
	}
	if tc.APIKey == "" {
		tc.APIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if tc.APIKey == "" {
		tc.APIKey = loadCodexAPIKey()
	}
	if tc.Model == "" {
		tc.Model = "gpt-4o-mini"
	}
	if tc.Language == "" {
		tc.Language = "Chinese"
	}

	return tc
}

func codexAuthPath() string {
	if dir := strings.TrimSpace(os.Getenv("CODEX_HOME")); dir != "" {
		return filepath.Join(dir, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex", "auth.json")
}

func loadCodexAPIKey() string {
	path := codexAuthPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var auth struct {
		OPENAIAPIKey *string `json:"OPENAI_API_KEY"`
	}
	if err := json.Unmarshal(data, &auth); err != nil || auth.OPENAIAPIKey == nil {
		return ""
	}
	return strings.TrimSpace(*auth.OPENAIAPIKey)
}
