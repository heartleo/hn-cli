package cli

import (
	"bytes"
	stdcolor "image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	hn "github.com/heartleo/hn-cli"
)

func preserveThemeState(t *testing.T) {
	t.Helper()
	theme := currentTheme
	name := currentThemeName
	isAuto := currentThemeIsAuto
	usesLightBackground := currentThemeUsesLightBackground
	styles := storyStyles
	t.Cleanup(func() {
		currentTheme = theme
		currentThemeName = name
		currentThemeIsAuto = isAuto
		currentThemeUsesLightBackground = usesLightBackground
		storyStyles = styles
	})
}

func sameColor(a, b stdcolor.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestResolveThemeDefaultsToAuto(t *testing.T) {
	preserveThemeState(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(themeEnvVar, "")

	resolveTheme()

	if currentThemeName != autoThemeName {
		t.Fatalf("expected default theme %q, got %q", autoThemeName, currentThemeName)
	}
	if !currentThemeIsAuto {
		t.Fatal("expected default theme to use terminal auto detection")
	}
}

func TestConfiguredThemeNameUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(themeEnvVar, " Nord ")

	if got := configuredThemeName(); got != "nord" {
		t.Fatalf("expected env theme override, got %q", got)
	}
}

func TestResolveThemeFallsBackToAutoForInvalidConfiguredTheme(t *testing.T) {
	preserveThemeState(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(themeEnvVar, "missing-theme")

	resolveTheme()

	if currentThemeName != autoThemeName {
		t.Fatalf("expected invalid configured theme to fall back to %q, got %q", autoThemeName, currentThemeName)
	}
	if !currentThemeIsAuto {
		t.Fatal("expected invalid configured theme fallback to use terminal auto detection")
	}
}

func TestThemeCommandShowsResolvedCurrentTheme(t *testing.T) {
	preserveThemeState(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(themeEnvVar, "missing-theme")
	resolveTheme()

	var out bytes.Buffer
	themeCmd.SetOut(&out)
	t.Cleanup(func() { themeCmd.SetOut(nil) })

	if err := themeCmd.RunE(themeCmd, nil); err != nil {
		t.Fatalf("theme command returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Current theme:") || !strings.Contains(got, autoThemeName) {
		t.Fatalf("expected resolved current theme in output, got %q", got)
	}
	if strings.Contains(got, "missing-theme") {
		t.Fatalf("expected raw invalid configured theme to be hidden, got %q", got)
	}
}

func TestThemeNamesIncludesAuto(t *testing.T) {
	names := themeNames()
	if len(names) == 0 || names[0] != autoThemeName {
		t.Fatalf("expected %q first in sorted theme names, got %#v", autoThemeName, names)
	}
}

func TestAutoThemeAppliesTerminalBackground(t *testing.T) {
	preserveThemeState(t)
	if !setTheme(autoThemeName) {
		t.Fatal("expected auto theme to be accepted")
	}
	darkTitle := currentTheme.Title

	if !applyAutoTheme(false) {
		t.Fatal("expected auto theme to react to terminal background")
	}
	if sameColor(currentTheme.Title, darkTitle) {
		t.Fatal("expected light terminal background to use a different title color")
	}
	if !sameColor(currentTheme.Title, autoTheme(false).Title) {
		t.Fatal("expected current theme to match light auto palette")
	}
	if got := markdownStyleName(); got != "light" {
		t.Fatalf("expected light terminal background to use light markdown style, got %q", got)
	}
}

func TestExplicitThemeIgnoresTerminalBackground(t *testing.T) {
	preserveThemeState(t)
	if !setTheme("hn") {
		t.Fatal("expected hn theme to be accepted")
	}
	title := currentTheme.Title

	if applyAutoTheme(false) {
		t.Fatal("expected explicit theme to ignore terminal background")
	}
	if !sameColor(currentTheme.Title, title) {
		t.Fatal("expected explicit theme colors to remain unchanged")
	}
	if got := markdownStyleName(); got != "dark" {
		t.Fatalf("expected explicit theme to keep dark markdown style, got %q", got)
	}
}

func TestBackgroundColorMsgUpdatesAutoTheme(t *testing.T) {
	preserveThemeState(t)
	if !setTheme(autoThemeName) {
		t.Fatal("expected auto theme to be accepted")
	}
	m := newModel(hn.CategoryTop)
	darkTitle := currentTheme.Title

	updated, _ := m.Update(tea.BackgroundColorMsg{Color: stdcolor.White})
	_ = updated.(model)

	if sameColor(currentTheme.Title, darkTitle) {
		t.Fatal("expected light background message to update current theme")
	}
	if !sameColor(currentTheme.Title, lipgloss.Color("#1f2937")) {
		t.Fatalf("expected light palette title, got %#v", currentTheme.Title)
	}
}
