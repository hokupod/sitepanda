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

	"github.com/playwright-community/playwright-go"
)

const version = "0.0.4"
const lightpandaNightlyVersion = "nightly"

var (
	outfile            string
	matchPatterns      stringSlice
	pageLimit          int
	contentSelector    string
	silent             bool
	showVersion        bool
	waitForNetworkIdle bool
	browserName        string
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
	defaultBrowser := "lightpanda"
	envBrowser := os.Getenv("SITEPANDA_BROWSER")
	if envBrowser != "" {
		if envBrowser == "lightpanda" || envBrowser == "chromium" {
			defaultBrowser = envBrowser
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid value for SITEPANDA_BROWSER environment variable: %s. Using '%s' or command-line specified browser.\n", envBrowser, defaultBrowser)
		}
	}

	flag.StringVar(&browserName, "browser", defaultBrowser, "Browser to use for scraping ('lightpanda' or 'chromium')")
	flag.StringVar(&browserName, "b", defaultBrowser, "Browser to use for scraping ('lightpanda' or 'chromium') (shorthand for --browser)")
	flag.IntVar(&pageLimit, "limit", 0, "Limit the result to this amount of pages (0 for no limit)")
	flag.StringVar(&outfile, "outfile", "", "Write the fetched site to a text file")
	flag.StringVar(&outfile, "o", "", "Write the fetched site to a text file (shorthand)")
	flag.Var(&matchPatterns, "match", "Only extract content from matched pages (glob pattern, can be specified multiple times)")
	flag.Var(&matchPatterns, "m", "Only fetch matched pages (glob pattern, can be specified multiple times, shorthand)")
	flag.StringVar(&contentSelector, "content-selector", "", "CSS selector to find the main content area (e.g., .article-body)")
	flag.BoolVar(&silent, "silent", false, "Do not print any logs")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&waitForNetworkIdle, "wait-for-network-idle", false, "Wait for network to be idle instead of just load when fetching pages")
	flag.BoolVar(&waitForNetworkIdle, "wni", false, "Shorthand for --wait-for-network-idle")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [command] [options] <url>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Sitepanda is a CLI tool to scrape websites and save content as Markdown.\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  SITEPANDA_BROWSER         Specifies the default browser ('lightpanda' or 'chromium').\n")
		fmt.Fprintf(os.Stderr, "                            Command-line options --browser or -b will override this.\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init [lightpanda|chromium]  Download and install the specified browser dependency (default: lightpanda).\n")
		fmt.Fprintf(os.Stderr, "  <url>                     Start scraping from the given URL (default command if no other command is specified).\n\n")
		fmt.Fprintf(os.Stderr, "Options for scraping (when <url> is provided):\n")
		fmt.Fprintf(os.Stderr, "  --browser <name>, -b <name> Browser to use (lightpanda or chromium). Default: %s (or SITEPANDA_BROWSER value)\n", defaultBrowser)
		fmt.Fprintf(os.Stderr, "  -o, --outfile <path>          Write the fetched site to a text file. If path ends with .json, output is JSON.\n")
		fmt.Fprintf(os.Stderr, "  -m, --match <pattern>         Only extract content from matched pages (glob pattern, can be specified multiple times).\n")
		fmt.Fprintf(os.Stderr, "  --limit <number>              Stop crawling once this many pages have had their content saved (0 for no limit).\n")
		fmt.Fprintf(os.Stderr, "  --content-selector <selector> Specify a CSS selector to target the main content area.\n")
		fmt.Fprintf(os.Stderr, "  --wait-for-network-idle, -wni Wait for network to be idle instead of just load when fetching pages.\n")
		fmt.Fprintf(os.Stderr, "  --silent                      Do not print any logs.\n")
		fmt.Fprintf(os.Stderr, "  --version                     Show version information.\n")

		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s init\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s init chromium\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  SITEPANDA_BROWSER=chromium %s --outfile output.json https://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --browser chromium --outfile output.json https://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -b chromium --outfile output.json https://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --wait-for-network-idle --outfile output.json https://example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -wni --outfile output.json https://example.com\n", os.Args[0])
	}

	flag.Parse()

	if silent {
		logger.SetOutput(io.Discard)
	}

	if envBrowser != "" && (envBrowser == "lightpanda" || envBrowser == "chromium") {
		if !isFlagPassed("browser") && !isFlagPassed("b") {
			logger.Printf("Using browser from SITEPANDA_BROWSER environment variable: %s", browserName)
		}
	}

	args := flag.Args()

	if len(args) > 0 && args[0] == "init" {
		browserToInit := "lightpanda"
		if len(args) > 1 {
			browserToInit = args[1]
			if browserToInit != "lightpanda" && browserToInit != "chromium" {
				logger.Printf("Error: 'init' command supports 'lightpanda' or 'chromium' as an argument. Got: %s", browserToInit)
				flag.Usage()
				os.Exit(1)
			}
		}
		if len(args) > 2 {
			logger.Println("Error: 'init' command takes at most one argument (browser name: 'lightpanda' or 'chromium').")
			flag.Usage()
			os.Exit(1)
		}
		handleInitCommand(browserToInit)
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
	if startURL == "init" {
		logger.Println("Error: 'init' is a command, not a URL. To initialize, run 'sitepanda init [browser]'.")
		flag.Usage()
		os.Exit(1)
	}
	if browserName != "lightpanda" && browserName != "chromium" {
		logger.Printf("Error: Invalid browser specified: %s. Supported: 'lightpanda', 'chromium'. Check command-line options or SITEPANDA_BROWSER environment variable.", browserName)
		flag.Usage()
		os.Exit(1)
	}

	logger.Printf("Sitepanda v%s starting with browser: %s", version, browserName)

	playwrightDriverDir, err := getAppSubdirectory("playwright_driver")
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Printf("Received signal: %v. Shutting down...", sig)
		os.Exit(1)
	}()

	logger.Printf("Configuration:")
	logger.Printf("  URL: %s", startURL)
	logger.Printf("  Browser: %s", browserName)
	if browserName == "lightpanda" {
		logger.Printf("  Lightpanda Path: %s", browserExecutablePath)
		logger.Printf("  Lightpanda WebSocket: %s", wsURL)
	} else if browserName == "chromium" {
		logger.Printf("  Chromium managed by Playwright in: %s", playwrightDriverDir)
	}
	logger.Printf("  Outfile: %s", outfile)
	logger.Printf("  Match Patterns: %v", matchPatterns)
	logger.Printf("  Page Limit: %d", pageLimit)
	logger.Printf("  Content Selector: %s", contentSelector)
	logger.Printf("  Silent: %t", silent)
	logger.Printf("  Wait For Network Idle: %t", waitForNetworkIdle)

	var crawler *Crawler
	var crawlerErr error

	if browserName == "lightpanda" {
		crawler, crawlerErr = NewCrawlerForLightpanda(startURL, wsURL, pwInstance, pageLimit, matchPatterns, contentSelector, outfile, silent, waitForNetworkIdle)
	} else if browserName == "chromium" {
		crawler, crawlerErr = NewCrawlerForPlaywrightBrowser(startURL, pwBrowser, pageLimit, matchPatterns, contentSelector, outfile, silent, waitForNetworkIdle)
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

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func handleInitCommand(browserToInstall string) {
	logger.Printf("Initializing Sitepanda: Setting up %s...", browserToInstall)

	switch browserToInstall {
	case "lightpanda":
		lpExecutablePath, err := getBrowserExecutablePath("lightpanda")
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
				logger.Fatalf("Unsupported architecture for Lightpanda on Linux: %s. Lightpanda is available for linux/amd64.", runtime.GOARCH)
			}
		case "darwin":
			if runtime.GOARCH == "arm64" {
				lpFilename = "lightpanda-aarch64-macos"
				downloadURL = fmt.Sprintf("https://github.com/lightpanda-io/browser/releases/download/%s/%s", lightpandaNightlyVersion, lpFilename)
			} else {
				logger.Fatalf("Unsupported architecture for Lightpanda on macOS: %s. Lightpanda is primarily available for darwin/arm64.", runtime.GOARCH)
			}
		default:
			logger.Fatalf("Unsupported OS for Lightpanda: %s. Lightpanda can only be automatically installed on Linux (amd64) and macOS (arm64).", runtime.GOOS)
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
		err = os.WriteFile(lpExecutablePath, binaryData, 0755)
		if err != nil {
			logger.Fatalf("Failed to write Lightpanda executable to %s: %v", lpExecutablePath, err)
		}
		logger.Printf("Lightpanda downloaded and installed successfully to %s", lpExecutablePath)

	case "chromium":
		logger.Println("Setting up Chromium via playwright-go...")
		playwrightInstallDir, err := getAppSubdirectory("playwright_driver")
		if err != nil {
			logger.Fatalf("Failed to get or create Sitepanda's Playwright driver directory: %v", err)
		}
		logger.Printf("Playwright components (including Chromium) will be installed by Sitepanda into: %s", playwrightInstallDir)
		logger.Printf("This directory will be used as PLAYWRIGHT_DRIVER_PATH for Playwright operations.")

		installOptions := playwright.RunOptions{
			Browsers:        []string{"chromium"},
			DriverDirectory: playwrightInstallDir,
			Verbose:         true,
			Stdout:          os.Stdout,
			Stderr:          os.Stderr,
		}

		logger.Println("Running playwright.Install to download and set up Chromium...")
		if err := playwright.Install(&installOptions); err != nil {
			logger.Fatalf("Failed to install Chromium using playwright-go: %v", err)
		}
		logger.Println("Chromium has been successfully set up via playwright-go within Sitepanda's designated directory.")

		chromiumPathHint, pathErr := getBrowserExecutablePath("chromium", playwrightInstallDir)
		if pathErr != nil {
			logger.Printf("Note: Could not determine a specific path hint for the Chromium executable after installation: %v. This is generally okay as Playwright manages this internally within %s.", pathErr, playwrightInstallDir)
		} else {
			logger.Printf("A path hint for Chromium suggests it might be around: %s (Playwright handles actual execution path).", chromiumPathHint)
			if _, statErr := os.Stat(chromiumPathHint); statErr == nil {
				logger.Printf("The hinted Chromium path %s appears to exist.", chromiumPathHint)
			} else {
				logger.Printf("The hinted Chromium path %s could not be stat'd: %v. This might be okay.", chromiumPathHint, statErr)
			}
		}

	default:
		logger.Fatalf("Internal error: Unknown browser '%s' for init.", browserToInstall)
	}
	logger.Printf("Sitepanda initialization for %s complete.", browserToInstall)
}

func truncateString(s string, maxLen int) string {
	if maxLen < 0 {
		maxLen = 0
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
