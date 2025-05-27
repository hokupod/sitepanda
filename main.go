package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const version = "0.0.1"
const lightpandaNightlyVersion = "nightly"

var (
	outfile         string
	matchPatterns   stringSlice
	pageLimit       int
	contentSelector string
	silent          bool
	showVersion     bool
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	flag.IntVar(&pageLimit, "limit", 0, "Limit the result to this amount of pages (0 for no limit)")
	flag.StringVar(&outfile, "outfile", "", "Write the fetched site to a text file")
	flag.StringVar(&outfile, "o", "", "Write the fetched site to a text file (shorthand)")
	flag.Var(&matchPatterns, "match", "Only fetch matched pages (glob pattern, can be specified multiple times)")
	flag.Var(&matchPatterns, "m", "Only fetch matched pages (glob pattern, can be specified multiple times, shorthand)")
	flag.StringVar(&contentSelector, "content-selector", "", "CSS selector to find the main content area (e.g., .article-body)")
	flag.BoolVar(&silent, "silent", false, "Do not print any logs")
	flag.BoolVar(&showVersion, "version", false, "Show version information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [command] [options] <url>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Sitepanda is a CLI tool to scrape websites and save content as Markdown.\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init        Download and install the Lightpanda browser dependency.\n")
		fmt.Fprintf(os.Stderr, "  <url>       Start scraping from the given URL (default command if no other command is specified).\n\n")
		fmt.Fprintf(os.Stderr, "Options for scraping (when <url> is provided):\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s init\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --outfile output.json https://example.com\n", os.Args[0])
	}

	flag.Parse()

	if silent {
		logger.SetOutput(io.Discard)
	}

	args := flag.Args()

	if len(args) > 0 && args[0] == "init" {
		if len(args) > 1 {
			logger.Println("Error: 'init' command does not take any additional arguments.")
			flag.Usage()
			os.Exit(1)
		}
		handleInitCommand()
		return
	}

	if showVersion {
		fmt.Println(version)
		return
	}

	if len(args) < 1 {
		logger.Println("Error: URL argument is required for scraping, or specify 'init' command.")
		flag.Usage()
		os.Exit(1)
	}
	startURL := args[0]
	if startURL == "init" { // Should have been caught above, but as a safeguard
		logger.Println("Error: 'init' is a command, not a URL. To initialize, run 'sitepanda init'.")
		flag.Usage()
		os.Exit(1)
	}

	logger.Printf("Sitepanda v%s starting...", version)

	lpPath, lpCleanup, err := prepareLightpanda()
	if err != nil {
		logger.Fatalf("Failed to prepare Lightpanda: %v. If Lightpanda is not installed, please run 'sitepanda init'.", err)
	}
	defer lpCleanup()

	var lightpandaCmd *exec.Cmd
	var wsURL string
	var lpStdout, lpStderr *bytes.Buffer
	lightpandaCmd, wsURL, lpStdout, lpStderr, err = launchLightpandaServer(lpPath)
	if err != nil {
		logger.Fatalf("Failed to launch Lightpanda server: %v. Ensure Lightpanda is correctly installed (try 'sitepanda init').", err)
	}

	logger.Println("Waiting a moment for Lightpanda server to initialize...")
	time.Sleep(5 * time.Second)
	logger.Println("Initial wait for Lightpanda finished.")

	defer func() {
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Printf("Received signal: %v. Shutting down...", sig)
		// The deferred functions (lpCleanup, Lightpanda process termination) will be called on exit.
		os.Exit(1)
	}()

	logger.Printf("Configuration:")
	logger.Printf("  URL: %s", startURL)
	logger.Printf("  Outfile: %s", outfile)
	logger.Printf("  Match Patterns: %v", matchPatterns)
	logger.Printf("  Page Limit: %d", pageLimit)
	logger.Printf("  Content Selector: %s", contentSelector)
	logger.Printf("  Silent: %t", silent)
	logger.Printf("  Lightpanda Path: %s", lpPath)
	logger.Printf("  Lightpanda WebSocket: %s", wsURL)

	crawler, err := NewCrawler(startURL, wsURL, pageLimit, matchPatterns, contentSelector, outfile, silent)
	if err != nil {
		if lpStdout != nil && lpStdout.Len() > 0 {
			logger.Printf("--- Lightpanda stdout (on NewCrawler failure) ---\n%s", lpStdout.String())
		} else if lpStdout != nil {
			logger.Println("--- Lightpanda stdout (on NewCrawler failure) --- (empty)")
		}

		if lpStderr != nil && lpStderr.Len() > 0 {
			logger.Printf("--- Lightpanda stderr (on NewCrawler failure) ---\n%s", lpStderr.String())
		} else if lpStderr != nil {
			logger.Println("--- Lightpanda stderr (on NewCrawler failure) --- (empty)")
		}
		logger.Fatalf("Failed to initialize crawler: %v", err)
	}

	crawlErr := crawler.Crawl()
	if crawlErr != nil {
		logger.Printf("Crawling failed: %v", crawlErr)
		if lpStdout != nil {
			if lpStdout.Len() > 0 {
				logger.Printf("--- Lightpanda stdout (on Crawl failure) ---\n%s", lpStdout.String())
			} else {
				logger.Println("--- Lightpanda stdout (on Crawl failure) --- (empty)")
			}
		}
		if lpStderr != nil {
			if lpStderr.Len() > 0 {
				logger.Printf("--- Lightpanda stderr (on Crawl failure) ---\n%s", lpStderr.String())
			} else {
				logger.Println("--- Lightpanda stderr (on Crawl failure) --- (empty)")
			}
		}
	}

	logger.Println("Sitepanda finished.")
}

func handleInitCommand() {
	logger.Println("Initializing Sitepanda: Setting up Lightpanda...")

	lpExecutablePath, err := getLightpandaExecutablePath()
	if err != nil {
		logger.Fatalf("Error determining Lightpanda installation path: %v", err)
	}
	lpInstallDir := filepath.Dir(lpExecutablePath)

	logger.Printf("Lightpanda will be installed to: %s", lpExecutablePath)

	var downloadURL string
	var lpFilename string

	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			lpFilename = "lightpanda-x86_64-linux"
			downloadURL = fmt.Sprintf("https://github.com/lightpanda-io/browser/releases/download/%s/%s", lightpandaNightlyVersion, lpFilename)
		} else {
			logger.Fatalf("Unsupported architecture for Linux: %s. Lightpanda is available for linux/amd64.", runtime.GOARCH)
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			lpFilename = "lightpanda-aarch64-macos"
			downloadURL = fmt.Sprintf("https://github.com/lightpanda-io/browser/releases/download/%s/%s", lightpandaNightlyVersion, lpFilename)
		} else {
			logger.Fatalf("Unsupported architecture for macOS: %s. Lightpanda is available for darwin/arm64.", runtime.GOARCH)
		}
	default:
		logger.Fatalf("Unsupported OS: %s. Lightpanda can only be automatically installed on Linux (amd64) and macOS (arm64).", runtime.GOOS)
	}

	logger.Printf("Downloading Lightpanda for %s/%s from %s...", runtime.GOOS, runtime.GOARCH, downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		logger.Fatalf("Failed to download Lightpanda: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Fatalf("Failed to download Lightpanda: server returned status %s. Response: %s", resp.Status, string(bodyBytes))
	}

	// Ensure the target directory exists (getLightpandaExecutablePath's underlying getAppSubdirectory should do this, but double check)
	if err := os.MkdirAll(lpInstallDir, 0755); err != nil {
		logger.Fatalf("Failed to create installation directory %s: %v", lpInstallDir, err)
	}

	binaryData, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Fatalf("Failed to read Lightpanda binary data from response: %v", err)
	}

	if len(binaryData) == 0 {
		logger.Fatalf("Downloaded Lightpanda binary is empty. URL: %s", downloadURL)
	}

	err = os.WriteFile(lpExecutablePath, binaryData, 0755) // 0755 makes it rwxr-xr-x
	if err != nil {
		logger.Fatalf("Failed to write Lightpanda executable to %s: %v", lpExecutablePath, err)
	}

	logger.Printf("Lightpanda downloaded and installed successfully to %s", lpExecutablePath)
	logger.Println("Sitepanda initialization complete.")
}

func truncateString(s string, maxLen int) string {
	if maxLen < 0 {
		maxLen = 0 // Treat negative maxLen as 0
	}
	if maxLen == 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
