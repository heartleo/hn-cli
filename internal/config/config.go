package config

import (
	"encoding/base64"
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
	APIURL           string `json:"api_url,omitempty"`
	APIKey           string `json:"api_key,omitempty"`
	Model            string `json:"model,omitempty"`
	Language         string `json:"language,omitempty"`
	ReasoningEffort  string `json:"reasoning_effort,omitempty"`
	ServiceTier      string `json:"service_tier,omitempty"`
	WireAPI          string `json:"wire_api,omitempty"`
	AuthSource       string `json:"-"`
	ChatGPTAccountID string `json:"-"`
	SetupHint        string `json:"-"`
}

const (
	defaultTranslateAPIURL           = "https://api.openai.com/v1"
	defaultCodexOAuthAPIURL          = "https://chatgpt.com/backend-api/codex"
	defaultTranslateModel            = "gpt-4o-mini"
	defaultCodexOAuthTranslateModel  = "gpt-5.5"
	defaultCodexOAuthReasoningEffort = "none"
	defaultCodexOAuthServiceTier     = "priority"
	translateWireAPIChatCompletions  = "chat_completions"
	translateWireAPICodexResponses   = "codex_responses"
	translateAuthSourceConfigured    = "configured"
	translateAuthSourceOpenAIEnv     = "openai_env"
	translateAuthSourceCodexAPIKey   = "codex_api_key"
	translateAuthSourceCodexOAuth    = "codex_oauth"
)

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
	apiURLConfigured := strings.TrimSpace(tc.APIURL) != ""

	if v := os.Getenv("HN_TRANSLATE_API_URL"); v != "" {
		tc.APIURL = v
		apiURLConfigured = true
	}
	if v := os.Getenv("HN_TRANSLATE_API_KEY"); v != "" {
		tc.APIKey = v
		tc.AuthSource = translateAuthSourceConfigured
	}
	if v := os.Getenv("HN_TRANSLATE_MODEL"); v != "" {
		tc.Model = v
	}
	if v := os.Getenv("HN_TRANSLATE_LANG"); v != "" {
		tc.Language = v
	}
	if v := os.Getenv("HN_TRANSLATE_REASONING_EFFORT"); v != "" {
		tc.ReasoningEffort = v
	}
	if v := os.Getenv("HN_TRANSLATE_SERVICE_TIER"); v != "" {
		tc.ServiceTier = v
	}
	if v := os.Getenv("HN_TRANSLATE_WIRE_API"); v != "" {
		tc.WireAPI = v
	}

	if tc.APIURL == "" {
		tc.APIURL = defaultTranslateAPIURL
	}
	if tc.APIKey != "" && tc.AuthSource == "" {
		tc.AuthSource = translateAuthSourceConfigured
	}
	if tc.APIKey == "" {
		tc.APIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if tc.APIKey != "" {
			tc.AuthSource = translateAuthSourceOpenAIEnv
		}
	}
	if tc.APIKey == "" {
		var hint string
		tc.APIKey, tc.AuthSource, tc.ChatGPTAccountID, hint = loadCodexCredential()
		tc.SetupHint = hint
	}
	if tc.AuthSource == translateAuthSourceCodexOAuth && !apiURLConfigured {
		tc.APIURL = defaultCodexOAuthAPIURL
	}
	if tc.Model == "" {
		if tc.AuthSource == translateAuthSourceCodexOAuth {
			tc.Model = defaultCodexOAuthTranslateModel
		} else {
			tc.Model = defaultTranslateModel
		}
	}
	if tc.Language == "" {
		tc.Language = "Chinese"
	}
	if tc.AuthSource == translateAuthSourceCodexOAuth {
		if tc.WireAPI == "" {
			tc.WireAPI = translateWireAPICodexResponses
		}
		if tc.ReasoningEffort == "" {
			tc.ReasoningEffort = defaultCodexOAuthReasoningEffort
		}
		if tc.ServiceTier == "" {
			tc.ServiceTier = defaultCodexOAuthServiceTier
		} else if strings.EqualFold(tc.ServiceTier, "fast") {
			tc.ServiceTier = defaultCodexOAuthServiceTier
		}
	} else if tc.WireAPI == "" {
		tc.WireAPI = translateWireAPIChatCompletions
	}
	if tc.APIKey == "" && tc.SetupHint == "" {
		tc.SetupHint = "Set HN_TRANSLATE_API_KEY or OPENAI_API_KEY to enable translation."
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

func loadCodexCredential() (string, string, string, string) {
	path := codexAuthPath()
	if path == "" {
		return "", "", "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", ""
	}
	var auth struct {
		OPENAIAPIKey *string           `json:"OPENAI_API_KEY"`
		AuthMode     string            `json:"auth_mode"`
		Tokens       map[string]string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return "", "", "", ""
	}
	if auth.OPENAIAPIKey != nil {
		key := strings.TrimSpace(*auth.OPENAIAPIKey)
		if key != "" {
			return key, translateAuthSourceCodexAPIKey, "", ""
		}
	}
	if token := strings.TrimSpace(auth.Tokens["access_token"]); token != "" {
		accountID := strings.TrimSpace(auth.Tokens["account_id"])
		if accountID == "" {
			accountID = chatGPTAccountIDFromAccessToken(token)
		}
		return token, translateAuthSourceCodexOAuth, accountID, ""
	}
	if strings.EqualFold(auth.AuthMode, "chatgpt") || len(auth.Tokens) > 0 {
		return "", "", "", "Codex auth.json is signed in with ChatGPT but has no usable access token. Run codex login again or set HN_TRANSLATE_API_KEY."
	}
	return "", "", "", ""
}

func chatGPTAccountIDFromAccessToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		OpenAIAuth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.OpenAIAuth.ChatGPTAccountID)
}
