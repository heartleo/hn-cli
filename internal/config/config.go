package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds global CLI configuration.
type Config struct {
	Theme         string           `json:"theme,omitempty"`
	CommentSource string           `json:"comment_source,omitempty"`
	Translate     *TranslateConfig `json:"translate,omitempty"`
}

// Comment-source values. Algolia fetches an entire thread in one request;
// Firebase fans out per-item with two-phase subtree loading.
const (
	CommentSourceAlgolia  = "algolia"
	CommentSourceFirebase = "firebase"
)

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

// LoadCommentSource resolves the active comment source.
// Precedence: HN_COMMENT_SOURCE env var > config.json > default (algolia).
// Unknown values fall back to the default.
func LoadCommentSource() string {
	pick := func(v string) string {
		switch v {
		case CommentSourceAlgolia, CommentSourceFirebase:
			return v
		}
		return ""
	}
	if v := pick(os.Getenv("HN_COMMENT_SOURCE")); v != "" {
		return v
	}
	cfg, _ := LoadConfig()
	if v := pick(cfg.CommentSource); v != "" {
		return v
	}
	return CommentSourceAlgolia
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
	if tc.Model == "" {
		tc.Model = "gpt-4o-mini"
	}
	if tc.Language == "" {
		tc.Language = "Chinese"
	}

	return tc
}
