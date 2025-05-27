package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const lightpandaExecutableName = "lightpanda"

// userHomeDir is a variable to allow mocking os.UserHomeDir in tests.
var userHomeDir = os.UserHomeDir

// getAppSubdirectory creates and returns an application-specific directory path.
// Linux: $XDG_DATA_HOME/sitepanda/subPath... or ~/.local/share/sitepanda/subPath...
// macOS: ~/Library/Application Support/Sitepanda/subPath...
func getAppSubdirectory(subPath ...string) (string, error) {
	var baseDir string
	var err error
	var appDirName string

	switch runtime.GOOS {
	case "linux":
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome == "" {
			homeDir, err := userHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			baseDir = filepath.Join(homeDir, ".local", "share")
		} else {
			baseDir = xdgDataHome
		}
		appDirName = "sitepanda" // Lowercase 's' for Linux
	case "darwin": // macOS
		homeDir, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, "Library", "Application Support")
		appDirName = "Sitepanda" // Uppercase 'S' for macOS
	default:
		return "", fmt.Errorf("unsupported OS for Lightpanda installation: %s. Only Linux and macOS are supported", runtime.GOOS)
	}

	appSpecificPath := []string{baseDir, appDirName}
	appSpecificPath = append(appSpecificPath, subPath...)
	fullPath := filepath.Join(appSpecificPath...)

	err = os.MkdirAll(fullPath, 0755) // 0755 gives rwx for owner, rx for group/others
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// getLightpandaExecutablePathActual is the actual implementation of getLightpandaExecutablePath.
var getLightpandaExecutablePathActual = func() (string, error) {
	binDir, err := getAppSubdirectory("bin")
	if err != nil {
		return "", fmt.Errorf("failed to get application binary directory: %w", err)
	}
	return filepath.Join(binDir, lightpandaExecutableName), nil
}

// getLightpandaExecutablePath is a variable assigned the actual function,
// allowing it to be mocked for testing.
var getLightpandaExecutablePath = getLightpandaExecutablePathActual
