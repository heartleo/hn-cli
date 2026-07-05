package cli

import (
	"fmt"
	"image/color"
	"os"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/heartleo/hn-cli/internal/config"
	"github.com/spf13/cobra"
)

// Theme defines the color palette for the CLI.
type Theme struct {
	Accent  color.Color //tab highlight, spinner, cursor
	Link    color.Color //URLs, clickable elements
	Title   color.Color //story titles, primary content
	Success color.Color //checkmarks
	Error   color.Color //errors
	Warning color.Color //warnings
	Info    color.Color //hints
	Muted   color.Color //secondary text (author, time)
	Surface color.Color //selected row background
	Score   color.Color //score ▲
	Comment color.Color //comment count
}

var themes = map[string]Theme{
	"hn": {
		Accent:  lipgloss.Color("208"), // HN orange #FF6600
		Link:    lipgloss.Color("208"), // HN orange links
		Title:   lipgloss.Color("255"), // bright white
		Success: lipgloss.Color("114"),
		Error:   lipgloss.Color("204"),
		Warning: lipgloss.Color("223"),
		Info:    lipgloss.Color("109"),
		Muted:   lipgloss.Color("243"), // HN gray #828282
		Surface: lipgloss.Color("236"),
		Score:   lipgloss.Color("208"), // HN orange
		Comment: lipgloss.Color("243"),
	},
	"mocha": {
		Accent:  lipgloss.Color("183"),
		Link:    lipgloss.Color("111"),
		Title:   lipgloss.Color("189"),
		Success: lipgloss.Color("114"),
		Error:   lipgloss.Color("204"),
		Warning: lipgloss.Color("223"),
		Info:    lipgloss.Color("109"),
		Muted:   lipgloss.Color("243"),
		Surface: lipgloss.Color("238"),
		Score:   lipgloss.Color("208"),
		Comment: lipgloss.Color("109"),
	},
	"dracula": {
		Accent:  lipgloss.Color("141"),
		Link:    lipgloss.Color("117"),
		Title:   lipgloss.Color("231"),
		Success: lipgloss.Color("84"),
		Error:   lipgloss.Color("210"),
		Warning: lipgloss.Color("228"),
		Info:    lipgloss.Color("117"),
		Muted:   lipgloss.Color("61"),
		Surface: lipgloss.Color("236"),
		Score:   lipgloss.Color("208"),
		Comment: lipgloss.Color("117"),
	},
	"tokyo": {
		Accent:  lipgloss.Color("75"),
		Link:    lipgloss.Color("117"),
		Title:   lipgloss.Color("189"),
		Success: lipgloss.Color("108"),
		Error:   lipgloss.Color("203"),
		Warning: lipgloss.Color("223"),
		Info:    lipgloss.Color("73"),
		Muted:   lipgloss.Color("59"),
		Surface: lipgloss.Color("236"),
		Score:   lipgloss.Color("208"),
		Comment: lipgloss.Color("73"),
	},
	"nord": {
		Accent:  lipgloss.Color("110"),
		Link:    lipgloss.Color("110"),
		Title:   lipgloss.Color("253"),
		Success: lipgloss.Color("108"),
		Error:   lipgloss.Color("174"),
		Warning: lipgloss.Color("222"),
		Info:    lipgloss.Color("73"),
		Muted:   lipgloss.Color("60"),
		Surface: lipgloss.Color("236"),
		Score:   lipgloss.Color("208"),
		Comment: lipgloss.Color("73"),
	},
	"gruvbox": {
		Accent:  lipgloss.Color("208"),
		Link:    lipgloss.Color("109"),
		Title:   lipgloss.Color("223"),
		Success: lipgloss.Color("142"),
		Error:   lipgloss.Color("167"),
		Warning: lipgloss.Color("214"),
		Info:    lipgloss.Color("108"),
		Muted:   lipgloss.Color("245"),
		Surface: lipgloss.Color("237"),
		Score:   lipgloss.Color("208"),
		Comment: lipgloss.Color("108"),
	},
}

var currentTheme = themes["hn"]

const themeEnvVar = "HN_THEME"
const autoThemeName = "auto"

var currentThemeName = "hn"
var currentThemeIsAuto bool

func autoTheme(isDark bool) Theme {
	if isDark {
		return themes["hn"]
	}
	return Theme{
		Accent:  lipgloss.Color("#ff6600"),
		Link:    lipgloss.Color("#b45309"),
		Title:   lipgloss.Color("#1f2937"),
		Success: lipgloss.Color("#15803d"),
		Error:   lipgloss.Color("#b91c1c"),
		Warning: lipgloss.Color("#9a3412"),
		Info:    lipgloss.Color("#0f766e"),
		Muted:   lipgloss.Color("#6b7280"),
		Surface: lipgloss.Color("#fff2df"),
		Score:   lipgloss.Color("#ff6600"),
		Comment: lipgloss.Color("#6b7280"),
	}
}

func setTheme(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = autoThemeName
	}
	if name == autoThemeName {
		currentThemeName = autoThemeName
		currentThemeIsAuto = true
		currentTheme = autoTheme(true)
		refreshStoryDelegateStyles()
		return true
	}
	if t, ok := themes[name]; ok {
		currentThemeName = name
		currentThemeIsAuto = false
		currentTheme = t
		refreshStoryDelegateStyles()
		return true
	}
	return false
}

func applyAutoTheme(isDark bool) bool {
	if !currentThemeIsAuto {
		return false
	}
	currentTheme = autoTheme(isDark)
	refreshStoryDelegateStyles()
	return true
}

func configuredThemeName() string {
	if env, ok := os.LookupEnv(themeEnvVar); ok && env != "" {
		return env
	}
	if cfg, err := config.LoadConfig(); err == nil && cfg.Theme != "" {
		return cfg.Theme
	}
	return autoThemeName
}

func resolveTheme() {
	setTheme(configuredThemeName())
}

func themeNames() []string {
	names := make([]string, 0, len(themes)+1)
	names = append(names, autoThemeName)
	for k := range themes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

var themeCmd = &cobra.Command{
	Use:   "theme [name]",
	Short: "Show or set color theme",
	Long:  "Show current theme or set it globally. Available: " + strings.Join(themeNames(), ", "),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Printf("Current theme: %s\n", colorBold(configuredThemeName()))
			fmt.Printf("Available: %s\n", strings.Join(themeNames(), ", "))
			return nil
		}

		name := strings.ToLower(args[0])
		if _, ok := themes[name]; !ok && name != autoThemeName {
			return fmt.Errorf("unknown theme %q, available: %s", name, strings.Join(themeNames(), ", "))
		}

		cfg, _ := config.LoadConfig()
		cfg.Theme = name
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save theme: %w", err)
		}

		setTheme(name)
		fmt.Printf("%s Theme set to %s\n", colorGreen(symbolSuccess), colorBold(name))
		fmt.Printf("%s Saved to: %s\n", colorFaint(symbolInfo), tildePath(config.ConfigPath()))
		return nil
	},
}
