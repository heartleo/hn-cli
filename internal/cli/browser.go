package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

var errBrowserOpenerUnavailable = errors.New("no browser opener available")

func openBrowser(url string) error {
	name, args, err := browserOpenCommand(runtime.GOOS, url, exec.LookPath)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start()
}

type lookPathFunc func(string) (string, error)

func browserOpenCommand(goos, url string, lookPath lookPathFunc) (string, []string, error) {
	var candidates [][]string
	switch goos {
	case "darwin":
		candidates = [][]string{{"open", url}}
	case "linux":
		candidates = [][]string{
			{"xdg-open", url},
			{"sensible-browser", url},
			{"gio", "open", url},
			{"wslview", url},
		}
	case "windows":
		candidates = [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}}
	default:
		return "", nil, fmt.Errorf("unsupported platform: %s", goos)
	}

	for _, candidate := range candidates {
		if _, err := lookPath(candidate[0]); err == nil {
			return candidate[0], candidate[1:], nil
		}
	}

	return "", nil, fmt.Errorf("%w: open this URL manually: %s", errBrowserOpenerUnavailable, url)
}
