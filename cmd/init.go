package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [browser]",
	Short: "Download and install browser dependencies",
	Long: `Download and install the specified browser dependency.

Supported browsers:
- chromium (default): Downloads Chromium via Playwright
- lightpanda: Downloads Lightpanda binary

The browser will be installed to a user-specific data directory and can be used for scraping.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		browserToInit := "chromium"
		if len(args) > 0 {
			browserToInit = args[0]
			if browserToInit != "lightpanda" && browserToInit != "chromium" {
				fmt.Fprintf(os.Stderr, "Error: 'init' command supports 'lightpanda' or 'chromium' as an argument. Got: %s\n", browserToInit)
				cmd.Usage()
				os.Exit(1)
			}
		}
		
		handleInitCommand(browserToInit)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// InitHandler is a function that handles browser initialization
// It will be set by the main package
var InitHandler func(string)

// handleInitCommand installs the specified browser
func handleInitCommand(browserToInstall string) {
	if InitHandler != nil {
		InitHandler(browserToInstall)
	} else {
		fmt.Printf("Error: Init handler not set. Please report this issue.\n")
		os.Exit(1)
	}
}