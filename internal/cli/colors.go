package cli

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	symbolSuccess = "✓"
	symbolError   = "✗"
	symbolInfo    = "→"
)

func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func colorGreen(s string) string {
	return lipgloss.NewStyle().Foreground(currentTheme.Success).Render(s)
}

func colorRed(s string) string {
	return lipgloss.NewStyle().Foreground(currentTheme.Error).Render(s)
}

func colorBold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

func colorFaint(s string) string {
	return lipgloss.NewStyle().Foreground(currentTheme.Muted).Render(s)
}
