package cli

import (
	"fmt"
	"log/slog"
	"os"
)

const debugLogFile = "debug.log"

// initDebug opens debug.log in the current directory and sets it as the
// slog default handler at Debug level. All slog.Debug calls in the process
// (including client.go) are written there for the lifetime of the program.
func initDebug() error {
	f, err := os.Create(debugLogFile)
	if err != nil {
		return fmt.Errorf("open debug log: %w", err)
	}
	h := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	slog.Debug("debug log opened", "file", debugLogFile)
	return nil
}
