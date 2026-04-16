package cli

import (
	"errors"
	"os/exec"
	"testing"
)

func TestBrowserOpenCommandUsesFirstAvailableLinuxOpener(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "gio" {
			return "/usr/bin/gio", nil
		}
		return "", exec.ErrNotFound
	}

	name, args, err := browserOpenCommand("linux", "https://example.com", lookPath)
	if err != nil {
		t.Fatalf("expected opener command, got error: %v", err)
	}
	if name != "gio" {
		t.Fatalf("expected gio, got %q", name)
	}
	if len(args) != 2 || args[0] != "open" || args[1] != "https://example.com" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBrowserOpenCommandReportsMissingOpener(t *testing.T) {
	lookPath := func(string) (string, error) {
		return "", exec.ErrNotFound
	}

	_, _, err := browserOpenCommand("linux", "https://example.com", lookPath)
	if !errors.Is(err, errBrowserOpenerUnavailable) {
		t.Fatalf("expected errBrowserOpenerUnavailable, got %v", err)
	}
}
