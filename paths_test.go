package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Mocks userHomeDir to return fakeHome for testing, restores after test.
func mockUserHomeDir(t *testing.T, fakeHome string) func() {
	t.Helper()
	originalUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) {
		return fakeHome, nil
	}
	return func() {
		userHomeDir = originalUserHomeDir
	}
}

func TestGetAppSubdirectory(t *testing.T) {
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDGDataHome)

	tests := []struct {
		name            string
		goos            string // Simulate runtime.GOOS for path logic
		xdgDataHomeVal  string // Linux XDG_DATA_HOME simulation
		useRelativeXDG  bool   // If xdgDataHomeVal is relative to fakeHome
		subPath         []string
		expectedBase    string // Expected path part relative to fakeHome or xdgDataHomeVal
		expectedFullDir string // Expected absolute path (if xdgDataHomeVal is absolute)
		expectError     bool
	}{
		{
			name:            "Linux with XDG_DATA_HOME set (absolute)",
			goos:            "linux",
			xdgDataHomeVal:  "/tmp/custom_xdg_data_abs_sitepanda",
			subPath:         []string{"bin"},
			expectedFullDir: "/tmp/custom_xdg_data_abs_sitepanda/sitepanda/bin",
		},
		{
			name:           "Linux with XDG_DATA_HOME set (relative to fake home)",
			goos:           "linux",
			xdgDataHomeVal: "custom_xdg_data_rel_sitepanda",
			useRelativeXDG: true,
			subPath:        []string{"bin"},
			expectedBase:   "custom_xdg_data_rel_sitepanda/sitepanda/bin",
		},
		{
			name:           "Linux XDG_DATA_HOME not set",
			goos:           "linux",
			xdgDataHomeVal: "",
			subPath:        []string{"config"},
			expectedBase:   ".local/share/sitepanda/config",
		},
		{
			name:         "macOS",
			goos:         "darwin",
			subPath:      []string{"cache"},
			expectedBase: "Library/Application Support/Sitepanda/cache",
		},
		{
			name:        "Unsupported OS (windows) for GetAppSubdirectory",
			goos:        "windows",
			expectError: true,
		},
		{
			name:            "No subPath (Linux with XDG_DATA_HOME)",
			goos:            "linux",
			xdgDataHomeVal:  "/tmp/data_no_subpath_sitepanda",
			subPath:         nil,
			expectedFullDir: "/tmp/data_no_subpath_sitepanda/sitepanda",
		},
		{
			name:           "Sitepanda playwright_driver subdir Linux",
			goos:           "linux",
			xdgDataHomeVal: "",
			subPath:        []string{"playwright_driver"},
			expectedBase:   ".local/share/sitepanda/playwright_driver",
		},
		{
			name:         "Sitepanda playwright_driver subdir macOS",
			goos:         "darwin",
			subPath:      []string{"playwright_driver"},
			expectedBase: "Library/Application Support/Sitepanda/playwright_driver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only run OS-specific test cases on the matching OS.
			if tt.goos != "" && tt.goos != runtime.GOOS {
				t.Skipf("Skipping OS-specific test case %s (for OS '%s') on '%s'", tt.name, tt.goos, runtime.GOOS)
				return
			}

			fakeHome := t.TempDir()
			cleanupHomeMock := mockUserHomeDir(t, fakeHome)
			defer cleanupHomeMock()

			// Set XDG_DATA_HOME only for Linux tests on Linux.
			if tt.goos == "linux" && runtime.GOOS == "linux" {
				if tt.xdgDataHomeVal != "" {
					if tt.useRelativeXDG {
						os.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, tt.xdgDataHomeVal))
					} else {
						os.Setenv("XDG_DATA_HOME", tt.xdgDataHomeVal)
					}
				} else {
					os.Unsetenv("XDG_DATA_HOME")
				}
			}

			path, err := GetAppSubdirectory(tt.subPath...)

			if (err != nil) != tt.expectError {
				t.Fatalf("GetAppSubdirectory() error = %v, wantErr %v. Path: %s. OS: %s, test case OS: %s", err, tt.expectError, path, runtime.GOOS, tt.goos)
			}

			if !tt.expectError {
				if path == "" {
					t.Errorf("GetAppSubdirectory() returned empty path")
				}

				var expectedPath string
				if tt.expectedFullDir != "" { // If absolute path is given (e.g. XDG_DATA_HOME is absolute)
					expectedPath = tt.expectedFullDir
				} else { // Otherwise, path is relative to fakeHome
					expectedPath = filepath.Join(fakeHome, tt.expectedBase)
				}

				if path != expectedPath {
					t.Errorf("GetAppSubdirectory() path = %q, want %q", path, expectedPath)
				}

				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					t.Errorf("GetAppSubdirectory() did not create directory: %s", path)
				}
			} else {
				// For Windows, check for the specific unsupported OS error message.
				if tt.goos == "windows" && runtime.GOOS == "windows" {
					if err == nil || !strings.Contains(err.Error(), "unsupported OS for general app subdirectory: windows") {
						t.Errorf("Expected error for Windows to contain 'unsupported OS for general app subdirectory: windows', got: %v", err)
					}
				}
			}
		})
	}
}

// Tests GetBrowserExecutablePath for various browsers and OSes.
func TestGetBrowserExecutablePath(t *testing.T) {
	fakeHome := t.TempDir()
	cleanupHomeMock := mockUserHomeDir(t, fakeHome)
	defer cleanupHomeMock()

	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDGDataHome)

	tests := []struct {
		name            string
		browserName     string
		baseInstallDirs []string // For Chromium, the Playwright DriverDirectory
		expectedSubPath string   // Expected path part relative to Sitepanda's app dir (for Lightpanda) or a hint structure (for Chromium)
		expectError     bool
		errorContains   string // Substring expected in error message
		targetOS        string // OS this test case is primarily for (linux, darwin)
	}{
		// Lightpanda cases
		{
			name:            "Lightpanda path on Linux",
			browserName:     "lightpanda",
			expectedSubPath: ".local/share/sitepanda/bin/lightpanda",
			targetOS:        "linux",
		},
		{
			name:            "Lightpanda path on macOS",
			browserName:     "lightpanda",
			expectedSubPath: "Library/Application Support/Sitepanda/bin/lightpanda",
			targetOS:        "darwin",
		},
		// Chromium cases (testing the unreliable hint nature)
		{
			name:            "Chromium path hint on Linux - expects error",
			browserName:     "chromium",
			baseInstallDirs: []string{filepath.Join(fakeHome, "playwright_driver_test_linux")},
			expectError:     true, // As GetBrowserExecutablePath for chromium returns an error indicating unreliability
			errorContains:   "path hint for Chromium",
			targetOS:        "linux",
		},
		{
			name:            "Chromium path hint on macOS - expects error",
			browserName:     "chromium",
			baseInstallDirs: []string{filepath.Join(fakeHome, "playwright_driver_test_macos")},
			expectError:     true,
			errorContains:   "path hint for Chromium",
			targetOS:        "darwin",
		},
		{
			name:            "Chromium path without baseInstallDir - expects error",
			browserName:     "chromium",
			baseInstallDirs: []string{}, // Empty or nil
			expectError:     true,
			errorContains:   "baseInstallDir (Playwright DriverDirectory) is required",
			targetOS:        runtime.GOOS, // Applicable to any OS
		},
		// Unsupported browser
		{
			name:          "Unsupported browser name",
			browserName:   "firefox", // Assuming firefox is not yet supported by this function
			expectError:   true,
			errorContains: "unsupported browser name for path: firefox",
			targetOS:      runtime.GOOS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only run test on the intended OS.
			if tt.targetOS != runtime.GOOS {
				t.Skipf("Skipping test %s on %s; test is for %s", tt.name, runtime.GOOS, tt.targetOS)
			}

			// Create base directory for Playwright if testing Chromium path hint.
			if tt.browserName == "chromium" && len(tt.baseInstallDirs) > 0 && tt.baseInstallDirs[0] != "" {
				if err := os.MkdirAll(tt.baseInstallDirs[0], 0755); err != nil {
					t.Fatalf("Failed to create mock Playwright base dir %s: %v", tt.baseInstallDirs[0], err)
				}
			}

			path, err := GetBrowserExecutablePath(tt.browserName, tt.baseInstallDirs...)

			if tt.expectError {
				if err == nil {
					t.Fatalf("GetBrowserExecutablePath() expected error for %s, but got nil. Path: %s", tt.name, path)
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("GetBrowserExecutablePath() error = %q, want error containing %q", err.Error(), tt.errorContains)
				}
			} else { // Not expecting error
				if err != nil {
					t.Fatalf("GetBrowserExecutablePath() for %s error = %v, want nil. Path: %s", tt.name, err, path)
				}

				var expectedFullPath string
				if tt.browserName == "lightpanda" {
					// For Lightpanda, path is relative to fakeHome (Sitepanda's app data structure)
					expectedFullPath = filepath.Join(fakeHome, tt.expectedSubPath)
				} else if tt.browserName == "chromium" {
					// Chromium: success case is not expected with current implementation.
					expectedFullPath = "UNEXPECTED_SUCCESS_PATH_FOR_CHROMIUM"
				}

				if path != expectedFullPath {
					t.Errorf("GetBrowserExecutablePath() = %q, want %q", path, expectedFullPath)
				}

				// For Lightpanda, check if the parent directory was created by getAppSubdirectory.
				if tt.browserName == "lightpanda" {
					if _, statErr := os.Stat(filepath.Dir(path)); os.IsNotExist(statErr) {
						t.Errorf("GetBrowserExecutablePath() did not create directory for Lightpanda: %s", filepath.Dir(path))
					}
				}
			}
		})
	}
	// Cleanup of fakeHome is handled by t.TempDir()
}
