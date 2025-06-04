package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"


var (
	// Global flags
	browserName         string
	silent              bool
	showVersion         bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sitepanda",
	Short: "A CLI tool to scrape websites and save content as Markdown",
	Long: `Sitepanda is a command-line interface (CLI) tool written in Go. It is designed to 
scrape websites using a headless browser (Chromium by default, or Lightpanda, controlled 
via Playwright), starting from a user-provided URL or a list of URLs from a file. The 
primary goal is to extract the main readable content from web pages and save it as Markdown.

Commands:
  init    Download and install browser dependencies
  scrape  Scrape websites and save content as Markdown`,
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Println(version)
			return
		}
		
		// Show help if no subcommand is provided
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	defaultBrowser := "chromium"
	if envBrowser := os.Getenv("SITEPANDA_BROWSER"); envBrowser != "" {
		if envBrowser == "lightpanda" || envBrowser == "chromium" {
			defaultBrowser = envBrowser
		}
	}
	
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&browserName, "browser", "b", defaultBrowser, "Browser to use for scraping ('lightpanda' or 'chromium')")
	rootCmd.PersistentFlags().BoolVar(&silent, "silent", false, "Do not print any logs")
	
	// Root command specific flags
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show version information")
}

// Getter functions for main package to access global flag values
func GetBrowserName() string { return browserName }
func GetSilent() bool        { return silent }