package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const defaultLightpandaExecutableName = "lightpanda"

// userHomeDir is a variable to allow mocking os.UserHomeDir in tests.
var userHomeDir = os.UserHomeDir

func GetAppSubdirectory(subPath ...string) (string, error) {
	var baseDir string
	var err error

	var appDirName string

	switch runtime.GOOS {
	case "linux":
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome == "" {
			homeDir, errHome := userHomeDir()
			if errHome != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", errHome)
			}
			baseDir = filepath.Join(homeDir, ".local", "share")
		} else {
			baseDir = xdgDataHome
		}
		appDirName = "sitepanda"
	case "darwin": // macOS
		homeDir, errHome := userHomeDir()
		if errHome != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", errHome)
		}
		baseDir = filepath.Join(homeDir, "Library", "Application Support")
		appDirName = "Sitepanda"
	case "windows":
		userConfigDir, errUserConfig := os.UserConfigDir()
		if errUserConfig != nil {
			return "", fmt.Errorf("failed to get user config directory: %w", errUserConfig)
		}
		baseDir = userConfigDir
		appDirName = "Sitepanda" // Consistent with macOS
	default:
		return "", fmt.Errorf("unsupported OS for general app subdirectory: %s. This function is for Sitepanda's own directories like 'bin' or 'playwright_driver'", runtime.GOOS)
	}

	appSpecificPathElements := []string{baseDir, appDirName}
	appSpecificPathElements = append(appSpecificPathElements, subPath...)
	fullPath := filepath.Join(appSpecificPathElements...)

	err = os.MkdirAll(fullPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// getBrowserExecutablePathActual is the actual implementation of getBrowserExecutablePath.
// For Chromium, baseInstallDir is the Playwright DriverDirectory.
// Returns a path hint for Chromium. Playwright itself is responsible for finding the exact executable.
var getBrowserExecutablePathActual = func(browserName string, baseInstallDir ...string) (string, error) {
	switch browserName {
	case "lightpanda":
		// Lightpanda is installed in Sitepanda's own 'bin' directory.
		sitepandaBinDir, err := GetAppSubdirectory("bin")
		if err != nil {
			return "", fmt.Errorf("failed to get application binary directory for Lightpanda: %w", err)
		}
		return filepath.Join(sitepandaBinDir, defaultLightpandaExecutableName), nil
	case "chromium":
		if len(baseInstallDir) == 0 || baseInstallDir[0] == "" {
			return "", fmt.Errorf("baseInstallDir (Playwright DriverDirectory) is required for Chromium path hint")
		}

		var playwrightVersion string
		playwrightVersion = "unknown-pw-version"

		var chromiumPathHint string
		switch runtime.GOOS {
		case "linux":

			chromiumPathHint = filepath.Join(baseInstallDir[0], "ms-playwright", playwrightVersion, "chromium", "linux", "chrome")
		case "darwin":

			chromiumPathHint = filepath.Join(baseInstallDir[0], "ms-playwright", playwrightVersion, "chromium", "mac", "Chromium.app", "Contents", "MacOS", "Chromium")
		case "windows":

			chromiumPathHint = filepath.Join(baseInstallDir[0], "ms-playwright", playwrightVersion, "chromium", "win64", "chrome.exe")
		default:
			return "", fmt.Errorf("chromium path hint generation not supported for OS: %s", runtime.GOOS)
		}

		return chromiumPathHint, fmt.Errorf("path hint for Chromium (%s) is unreliable; Playwright should auto-detect. Error returned to indicate this", chromiumPathHint)
	default:
		return "", fmt.Errorf("unsupported browser name for path: %s", browserName)
	}
}

// GetBrowserExecutablePath is assigned to the actual function to allow mocking in tests.
var GetBrowserExecutablePath = getBrowserExecutablePathActual
