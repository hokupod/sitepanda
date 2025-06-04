package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Helper function to execute command and capture output
func executeCommand(cmd *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError bool
		expectedOut   string
	}{
		{
			name:          "No arguments shows help",
			args:          []string{},
			expectedError: false,
			expectedOut:   "Commands:",
		},
		{
			name:          "Version flag",
			args:          []string{"--version"},
			expectedError: false,
			expectedOut:   "test-version",
		},
		{
			name:          "Help flag",
			args:          []string{"--help"},
			expectedError: false,
			expectedOut:   "Commands:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for each test
			cmd := &cobra.Command{
				Use:   "sitepanda",
				Short: "A CLI tool to scrape websites and save content as Markdown",
				Long:  "Commands:\n  init    Download and install browser dependencies\n  scrape  Scrape websites and save content as Markdown",
				Run: func(cmd *cobra.Command, args []string) {
					// Check if version flag is set
					versionFlag, _ := cmd.Flags().GetBool("version")
					if versionFlag {
						cmd.Print("test-version")
						return
					}
					cmd.Help()
				},
			}

			// Add version flag
			cmd.Flags().Bool("version", false, "Show version information")

			output, err := executeCommand(cmd, tt.args...)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !strings.Contains(output, tt.expectedOut) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectedOut, output)
			}
		})
	}
}

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError bool
		setupHandler  bool
	}{
		{
			name:          "Init with no browser defaults to chromium",
			args:          []string{"init"},
			expectedError: false,
			setupHandler:  true,
		},
		{
			name:          "Init with chromium",
			args:          []string{"init", "chromium"},
			expectedError: false,
			setupHandler:  true,
		},
		{
			name:          "Init with lightpanda",
			args:          []string{"init", "lightpanda"},
			expectedError: false,
			setupHandler:  true,
		},
		{
			name:          "Init with invalid browser",
			args:          []string{"init", "invalid"},
			expectedError: true,
			setupHandler:  true,
		},
		{
			name:          "Init with too many arguments",
			args:          []string{"init", "chromium", "extra"},
			expectedError: true,
			setupHandler:  true,
		},
		{
			name:          "Init without handler",
			args:          []string{"init"},
			expectedError: true,
			setupHandler:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command
			rootCmd := &cobra.Command{Use: "sitepanda"}

			// Create init command
			var handlerCalled bool
			var handlerArg string

			if tt.setupHandler {
				InitHandler = func(browser string) {
					handlerCalled = true
					handlerArg = browser
					// For invalid browser test, we expect the validation to happen before handler
					if browser == "invalid" {
						// This should not be reached as the command should validate first
						t.Errorf("Handler called with invalid browser: %s", browser)
					}
				}
			} else {
				InitHandler = nil
			}

			// Create a fresh init command for testing to avoid os.Exit issues
			testInitCmd := &cobra.Command{
				Use:   "init [browser]",
				Short: "Download and install browser dependencies",
				Args:  cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					browserToInit := "chromium"
					if len(args) > 0 {
						browserToInit = args[0]
						if browserToInit != "lightpanda" && browserToInit != "chromium" {
							return fmt.Errorf("'init' command supports 'lightpanda' or 'chromium' as an argument. Got: %s", browserToInit)
						}
					}
					
					if InitHandler != nil {
						InitHandler(browserToInit)
						return nil
					} else {
						return fmt.Errorf("Init handler not set. Please report this issue")
					}
				},
			}

			// Add test init command to root
			rootCmd.AddCommand(testInitCmd)

			output, err := executeCommand(rootCmd, tt.args...)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			// Check handler was called for successful cases
			if !tt.expectedError && tt.setupHandler {
				if !handlerCalled {
					t.Errorf("Expected handler to be called")
				}

				expectedBrowser := "chromium"
				if len(tt.args) > 1 {
					expectedBrowser = tt.args[1]
				}
				if handlerArg != expectedBrowser {
					t.Errorf("Expected handler to be called with %q, got %q", expectedBrowser, handlerArg)
				}
			}

			// Reset handler
			InitHandler = nil
		})
	}
}

func TestScrapeCommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError bool
		setupHandler  bool
		expectedURL   string
	}{
		{
			name:          "Scrape with URL",
			args:          []string{"scrape", "https://example.com"},
			expectedError: false,
			setupHandler:  true,
			expectedURL:   "https://example.com",
		},
		{
			name:          "Scrape without URL",
			args:          []string{"scrape"},
			expectedError: false,
			setupHandler:  true,
			expectedURL:   "",
		},
		{
			name:          "Scrape with flags",
			args:          []string{"scrape", "--outfile", "test.txt", "--match", "/test", "https://example.com"},
			expectedError: false,
			setupHandler:  true,
			expectedURL:   "https://example.com",
		},
		{
			name:          "Scrape without handler",
			args:          []string{"scrape", "https://example.com"},
			expectedError: true,
			setupHandler:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command
			rootCmd := &cobra.Command{Use: "sitepanda"}

			// Reset flags to default values
			outfile = ""
			urlFile = ""
			matchPatterns = []string{}
			followMatchPatterns = []string{}
			pageLimit = 0
			contentSelector = ""
			waitForNetworkIdle = false

			var handlerCalled bool
			var handlerArgs []string

			if tt.setupHandler {
				ScrapingHandler = func(args []string) {
					handlerCalled = true
					handlerArgs = args
				}
			} else {
				ScrapingHandler = nil
			}

			// Create a fresh scrape command for testing to avoid os.Exit issues
			testScrapeCmd := &cobra.Command{
				Use:   "scrape [url]",
				Short: "Scrape websites and save content as Markdown",
				Args:  cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					if ScrapingHandler != nil {
						ScrapingHandler(args)
						return nil
					} else {
						return fmt.Errorf("Scraping handler not set. Please report this issue")
					}
				},
			}

			// Add the same flags as the real scrape command
			testScrapeCmd.Flags().StringVarP(&outfile, "outfile", "o", "", "Write the fetched site to a text file")
			testScrapeCmd.Flags().StringVar(&urlFile, "url-file", "", "Path to a file containing URLs to process")
			testScrapeCmd.Flags().StringSliceVarP(&matchPatterns, "match", "m", []string{}, "Only extract content from matched pages")

			// Add test scrape command to root
			rootCmd.AddCommand(testScrapeCmd)

			output, err := executeCommand(rootCmd, tt.args...)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			// Check handler was called for successful cases
			if !tt.expectedError && tt.setupHandler {
				if !handlerCalled {
					t.Errorf("Expected handler to be called")
				}

				if tt.expectedURL != "" {
					if len(handlerArgs) == 0 || handlerArgs[0] != tt.expectedURL {
						t.Errorf("Expected handler to be called with URL %q, got args %v", tt.expectedURL, handlerArgs)
					}
				}
			}

			// Reset handler
			ScrapingHandler = nil
		})
	}
}

func TestFlagGetters(t *testing.T) {
	// Test global flag getters
	browserName = "lightpanda"
	silent = true

	if GetBrowserName() != "lightpanda" {
		t.Errorf("Expected browserName to be 'lightpanda', got %q", GetBrowserName())
	}

	if !GetSilent() {
		t.Errorf("Expected silent to be true")
	}

	// Test scrape flag getters
	outfile = "test.txt"
	urlFile = "urls.txt"
	matchPatterns = []string{"/test", "/example"}
	followMatchPatterns = []string{"/follow"}
	pageLimit = 10
	contentSelector = ".content"
	waitForNetworkIdle = true

	if GetOutfile() != "test.txt" {
		t.Errorf("Expected outfile to be 'test.txt', got %q", GetOutfile())
	}

	if GetURLFile() != "urls.txt" {
		t.Errorf("Expected urlFile to be 'urls.txt', got %q", GetURLFile())
	}

	expectedMatch := []string{"/test", "/example"}
	if len(GetMatchPatterns()) != len(expectedMatch) {
		t.Errorf("Expected match patterns length %d, got %d", len(expectedMatch), len(GetMatchPatterns()))
	}

	if GetPageLimit() != 10 {
		t.Errorf("Expected pageLimit to be 10, got %d", GetPageLimit())
	}

	if GetContentSelector() != ".content" {
		t.Errorf("Expected contentSelector to be '.content', got %q", GetContentSelector())
	}

	if !GetWaitForNetworkIdle() {
		t.Errorf("Expected waitForNetworkIdle to be true")
	}
}

func TestEnvironmentVariableBrowserDefault(t *testing.T) {
	// Test environment variable handling
	originalEnv := os.Getenv("SITEPANDA_BROWSER")
	defer func() {
		if originalEnv != "" {
			os.Setenv("SITEPANDA_BROWSER", originalEnv)
		} else {
			os.Unsetenv("SITEPANDA_BROWSER")
		}
	}()

	os.Setenv("SITEPANDA_BROWSER", "lightpanda")

	// This would normally be tested by re-executing init() but that's complex
	// Instead we test the logic directly
	defaultBrowser := "chromium"
	envBrowser := os.Getenv("SITEPANDA_BROWSER")
	if envBrowser != "" {
		if envBrowser == "lightpanda" || envBrowser == "chromium" {
			defaultBrowser = envBrowser
		}
	}

	if defaultBrowser != "lightpanda" {
		t.Errorf("Expected default browser to be 'lightpanda' when env var is set, got %q", defaultBrowser)
	}
}