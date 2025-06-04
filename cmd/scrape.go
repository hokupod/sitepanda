package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Scraping flags
	outfile             string
	urlFile             string
	matchPatterns       []string
	followMatchPatterns []string
	pageLimit           int
	contentSelector     string
	waitForNetworkIdle  bool
)

// ScrapingHandler is a function that handles the scraping functionality
// It will be set by the main package
var ScrapingHandler func([]string)

// scrapeCmd represents the scrape command
var scrapeCmd = &cobra.Command{
	Use:   "scrape [url]",
	Short: "Scrape websites and save content as Markdown",
	Long: `Scrape websites using a headless browser (Chromium by default, or Lightpanda, controlled 
via Playwright), starting from a user-provided URL or a list of URLs from a file. The 
primary goal is to extract the main readable content from web pages and save it as Markdown.

Examples:
  sitepanda scrape https://example.com
  sitepanda scrape --outfile output.txt --match "/blog/**" https://example.com
  sitepanda scrape --url-file urls.txt --outfile output.json
  sitepanda scrape --browser chromium --outfile output.json https://example.com`,
	Args: cobra.MaximumNArgs(1), // Allow 0 or 1 positional argument (the URL)
	Run: func(cmd *cobra.Command, args []string) {
		// Handle scraping logic
		if ScrapingHandler != nil {
			ScrapingHandler(args)
		} else {
			fmt.Printf("Error: Scraping handler not set. Please report this issue.\n")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(scrapeCmd)

	// Scraping flags
	scrapeCmd.Flags().StringVarP(&outfile, "outfile", "o", "", "Write the fetched site to a text file. If path ends with .json, output is JSON")
	scrapeCmd.Flags().StringVar(&urlFile, "url-file", "", "Path to a file containing URLs to process (one per line). Overrides <url> argument")
	scrapeCmd.Flags().StringSliceVarP(&matchPatterns, "match", "m", []string{}, "Only extract content from matched pages (glob pattern, can be specified multiple times)")
	scrapeCmd.Flags().StringSliceVar(&followMatchPatterns, "follow-match", []string{}, "Only add links matching this glob pattern to the crawl queue (can be specified multiple times)")
	scrapeCmd.Flags().IntVar(&pageLimit, "limit", 0, "Stop crawling once this many pages have had their content saved (0 for no limit)")
	scrapeCmd.Flags().StringVar(&contentSelector, "content-selector", "", "Specify a CSS selector to target the main content area")
	scrapeCmd.Flags().BoolVarP(&waitForNetworkIdle, "wait-for-network-idle", "w", false, "Wait for network to be idle instead of just load when fetching pages")
	scrapeCmd.Flags().BoolVar(&waitForNetworkIdle, "wni", false, "Shorthand for --wait-for-network-idle")
}

// Getter functions for main package to access flag values
func GetOutfile() string             { return outfile }
func GetURLFile() string             { return urlFile }
func GetMatchPatterns() []string     { return matchPatterns }
func GetFollowMatchPatterns() []string { return followMatchPatterns }
func GetPageLimit() int              { return pageLimit }
func GetContentSelector() string     { return contentSelector }
func GetWaitForNetworkIdle() bool    { return waitForNetworkIdle }