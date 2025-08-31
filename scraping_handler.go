package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hokupod/sitepanda/cmd"
	"github.com/playwright-community/playwright-go"
)

// HandleScraping implements the main scraping logic - exported version for cmd package
func HandleScraping(args []string) {
	// Configure logger based on silent flag
	if cmd.GetSilent() {
		SetLoggerOutput(io.Discard)
	}

	var startURLForCrawler string
	var targetURLsForCrawler []string
	isURLListMode := false

	// Handle URL arguments and --url-file logic (from original main.go lines 150-184)
	urlFile := cmd.GetURLFile()
	if urlFile != "" {
		if len(args) > 0 {
			logger.Fatal("Error: Cannot use <url> argument when --url-file is specified.")
		}
		fileContent, err := os.ReadFile(urlFile)
		if err != nil {
			logger.Fatalf("Error: Failed to read --url-file %s: %v", urlFile, err)
		}
		lines := strings.Split(string(fileContent), "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" {
				targetURLsForCrawler = append(targetURLsForCrawler, trimmedLine)
			}
		}
		if len(targetURLsForCrawler) == 0 {
			logger.Fatalf("Error: --url-file %s is empty or contains no valid URLs.", urlFile)
		}
		startURLForCrawler = targetURLsForCrawler[0]
		isURLListMode = true
	} else {
		if len(args) < 1 {
			logger.Println("Error: URL argument or --url-file option is required for scraping, or specify 'init' command.")
			os.Exit(1)
		}
		startURLForCrawler = args[0]
		if startURLForCrawler == "init" {
			logger.Println("Error: 'init' is a command, not a URL. To initialize, run 'sitepanda init [browser]'.")
			os.Exit(1)
		}
		targetURLsForCrawler = []string{startURLForCrawler}
		isURLListMode = false
	}

	// Validate browser name
	browserName := cmd.GetBrowserName()
	if browserName != "lightpanda" && browserName != "chromium" {
		logger.Printf("Error: Invalid browser specified: %s. Supported: 'lightpanda', 'chromium'. Check command-line options or SITEPANDA_BROWSER environment variable.", browserName)
		os.Exit(1)
	}

	// Early log for testing
	logger.Printf("Output Format: %s", cmd.GetOutputFormat())

	logger.Printf("Sitepanda v%s starting with browser: %s", Version, browserName)

	playwrightDriverDir, err := GetAppSubdirectory("playwright_driver")
	if err != nil {
		logger.Fatalf("Failed to determine or create Sitepanda's Playwright driver directory: %v", err)
	}

	browserExecutablePath, browserPrepareCleanup, err := prepareBrowser(browserName, playwrightDriverDir)
	if err != nil {
		logger.Fatalf("Failed to prepare %s: %v. If not installed, please run 'sitepanda init %s'.", browserName, err, browserName)
	}
	defer browserPrepareCleanup()

	var lightpandaCmd *exec.Cmd
	var wsURL string
	var pwInstance *playwright.Playwright
	var pwBrowser playwright.Browser
	var lpStdout, lpStderr *bytes.Buffer

	lightpandaCmd, wsURL, pwInstance, pwBrowser, lpStdout, lpStderr, err = launchBrowserAndGetConnection(browserName, browserExecutablePath, playwrightDriverDir)
	if err != nil {
		logger.Fatalf("Failed to launch %s or connect: %v.", browserName, err)
	}

	defer func() {
		if pwBrowser != nil && pwBrowser.IsConnected() {
			logger.Printf("Closing Playwright browser connection for %s...", browserName)
			if err := pwBrowser.Close(); err != nil {
				logger.Printf("Warning: failed to close Playwright browser for %s: %v", browserName, err)
			}
		}
		if pwInstance != nil {
			logger.Printf("Stopping Playwright instance for %s...", browserName)
			if err := pwInstance.Stop(); err != nil {
				logger.Printf("Warning: failed to stop Playwright instance for %s: %v", browserName, err)
			}
		}
		if lightpandaCmd != nil && lightpandaCmd.Process != nil {
			logger.Printf("Attempting to terminate Lightpanda process (PID: %d)...", lightpandaCmd.Process.Pid)
			if killErr := lightpandaCmd.Process.Kill(); killErr != nil {
				logger.Printf("Warning: failed to kill Lightpanda process (PID: %d): %v", lightpandaCmd.Process.Pid, killErr)
			} else {
				logger.Printf("Lightpanda process (PID: %d) terminated.", lightpandaCmd.Process.Pid)
			}
			_ = lightpandaCmd.Wait()
		}
	}()

	// Configuration logging
	outfile := cmd.GetOutfile()
	matchPatterns := cmd.GetMatchPatterns()
	followMatchPatterns := cmd.GetFollowMatchPatterns()
	pageLimit := cmd.GetPageLimit()
	contentSelector := cmd.GetContentSelector()
	waitForNetworkIdle := cmd.GetWaitForNetworkIdle()
	outputFormat := cmd.GetOutputFormat()

	logger.Printf("Configuration:")
	logger.Printf("  Start URL (or first from list): %s", startURLForCrawler)
	if isURLListMode {
		logger.Printf("  Mode: URL List from file (%s), %d URLs", urlFile, len(targetURLsForCrawler))
	} else {
		logger.Printf("  Mode: Single URL Crawl")
	}
	logger.Printf("  Browser: %s", browserName)
	if browserName == "lightpanda" {
		logger.Printf("  Lightpanda Path: %s", browserExecutablePath)
		logger.Printf("  Lightpanda WebSocket: %s", wsURL)
	} else if browserName == "chromium" {
		logger.Printf("  Chromium managed by Playwright in: %s", playwrightDriverDir)
	}
	logger.Printf("  Outfile: %s", outfile)
	logger.Printf("  Output Format: %s", outputFormat)
	logger.Printf("  Match Patterns (for content saving): %v", matchPatterns)
	if isURLListMode {
		logger.Printf("  Follow Match Patterns (for crawling): %v (ignored in URL list mode)", followMatchPatterns)
	} else {
		logger.Printf("  Follow Match Patterns (for crawling): %v", followMatchPatterns)
	}
	logger.Printf("  Page Limit: %d", pageLimit)
	logger.Printf("  Content Selector: %s", contentSelector)
	logger.Printf("  Silent: %t", cmd.GetSilent())
	logger.Printf("  Wait For Network Idle: %t", waitForNetworkIdle)

	var crawler *Crawler
	var crawlerErr error

	if browserName == "lightpanda" {
		crawler, crawlerErr = NewCrawlerForLightpanda(startURLForCrawler, targetURLsForCrawler, isURLListMode, wsURL, pwInstance, pageLimit, matchPatterns, followMatchPatterns, contentSelector, outfile, cmd.GetSilent(), waitForNetworkIdle, outputFormat)
	} else if browserName == "chromium" {
		crawler, crawlerErr = NewCrawlerForPlaywrightBrowser(startURLForCrawler, targetURLsForCrawler, isURLListMode, pwBrowser, pageLimit, matchPatterns, followMatchPatterns, contentSelector, outfile, cmd.GetSilent(), waitForNetworkIdle, outputFormat)
	} else {
		logger.Fatalf("Unsupported browser for crawler creation: %s", browserName)
	}

	if crawlerErr != nil {
		if lpStdout != nil && lpStdout.Len() > 0 {
			logger.Printf("--- Browser stdout (on NewCrawler failure) ---\n%s", lpStdout.String())
		}
		if lpStderr != nil && lpStderr.Len() > 0 {
			logger.Printf("--- Browser stderr (on NewCrawler failure) ---\n%s", lpStderr.String())
		}
		logger.Fatalf("Failed to initialize crawler: %v", crawlerErr)
	}

	// Setup signal handling for graceful shutdown with partial results
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Printf("Received signal: %v. Shutting down gracefully and saving partial results...", sig)
		crawler.Cancel()
	}()

	crawlErr := crawler.Crawl()
	if crawlErr != nil {
		logger.Printf("Crawling failed: %v", crawlErr)
		if lpStdout != nil && lpStdout.Len() > 0 {
			logger.Printf("--- Browser stdout (on Crawl failure) ---\n%s", lpStdout.String())
		}
		if lpStderr != nil && lpStderr.Len() > 0 {
			logger.Printf("--- Browser stderr (on Crawl failure) ---\n%s", lpStderr.String())
		}
	}

	logger.Println("Sitepanda finished.")
}
