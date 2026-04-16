package cli

import (
	"fmt"
	"os"

	hn "github.com/heartleo/hn-cli"
	"github.com/spf13/cobra"
)

var debugMode bool

var rootCmd = &cobra.Command{
	Use:           "hn",
	Short:         "A terminal client for Hacker News.",
	Long:          "A terminal client for Hacker News.",
	Version:       hn.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if debugMode {
			return initDebug()
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runApp(hn.CategoryTop)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\n", rootCmd.Name(), hn.Version)
	},
}

func categoryCmd(name string, cat hn.Category, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApp(cat)
		},
	}
}

func Execute() {
	if err := loadDotEnv(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		os.Exit(1)
	}
	resolveTheme()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		os.Exit(1)
	}
}

func formatError(err error) string {
	if err == nil {
		return ""
	}
	return colorRed(symbolError+" Error:") + " " + err.Error()
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "write debug log to debug.log")
	rootCmd.SetVersionTemplate("{{printf \"%s version %s\\n\" .Name .Version}}")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(categoryCmd("top", hn.CategoryTop, "Browse top stories"))
	rootCmd.AddCommand(categoryCmd("new", hn.CategoryNew, "Browse new stories"))
	rootCmd.AddCommand(categoryCmd("best", hn.CategoryBest, "Browse best stories"))
	rootCmd.AddCommand(categoryCmd("ask", hn.CategoryAsk, "Browse Ask HN"))
	rootCmd.AddCommand(categoryCmd("show", hn.CategoryShow, "Browse Show HN"))
	rootCmd.AddCommand(themeCmd)
}
