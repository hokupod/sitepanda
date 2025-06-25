package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/playwright-community/playwright-go"
)

// HandleInitCommand installs the specified browser - exported version for cmd package
func HandleInitCommand(browserToInstall string) {
	logger.Printf("Initializing Sitepanda: Setting up %s...", browserToInstall)

	switch browserToInstall {
	case "lightpanda":
		lpExecutablePath, err := GetBrowserExecutablePath("lightpanda")
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
				downloadURL = fmt.Sprintf("https://github.com/lightpanda-io/browser/releases/download/%s/%s", LightpandaNightlyVersion, lpFilename)
			} else {
				logger.Fatalf("Unsupported architecture for Lightpanda on Linux: %s. Lightpanda is available for linux/amd64.", runtime.GOARCH)
			}
		case "darwin":
			if runtime.GOARCH == "arm64" {
				lpFilename = "lightpanda-aarch64-macos"
				downloadURL = fmt.Sprintf("https://github.com/lightpanda-io/browser/releases/download/%s/%s", LightpandaNightlyVersion, lpFilename)
			} else {
				logger.Fatalf("Unsupported architecture for Lightpanda on macOS: %s. Lightpanda is primarily available for darwin/arm64.", runtime.GOARCH)
			}
		case "windows":
			logger.Fatalf("Lightpanda is not supported on Windows. Please use Chromium instead by running 'sitepanda init chromium'.")
		default:
			logger.Fatalf("Unsupported OS for Lightpanda: %s. Lightpanda can only be automatically installed on Linux (amd64) and macOS (arm64).", runtime.GOOS)
		}

		// This part will not be reached on Windows due to logger.Fatalf above.
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
		playwrightInstallDir, err := GetAppSubdirectory("playwright_driver")
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

		chromiumPathHint, pathErr := GetBrowserExecutablePath("chromium", playwrightInstallDir)
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
