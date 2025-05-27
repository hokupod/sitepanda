package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
		goos            string
		xdgDataHomeVal  string
		useRelativeXDG  bool
		subPath         []string
		expectedBase    string
		expectedFullDir string
		expectError     bool
	}{
		{
			name:            "Linux with XDG_DATA_HOME set (absolute)",
			goos:            "linux",
			xdgDataHomeVal:  "/tmp/custom_xdg_data_abs",
			subPath:         []string{"bin"},
			expectedFullDir: "/tmp/custom_xdg_data_abs/sitepanda/bin",
		},
		{
			name:           "Linux with XDG_DATA_HOME set (relative to fake home)",
			goos:           "linux",
			xdgDataHomeVal: "custom_xdg_data_rel",
			useRelativeXDG: true,
			subPath:        []string{"bin"},
			expectedBase:   "custom_xdg_data_rel/sitepanda/bin",
		},
		{
			name:           "Linux XDG_DATA_HOME not set",
			goos:           "linux",
			xdgDataHomeVal: "",
			subPath:        []string{"config"},
			expectedBase:   ".local/share/sitepanda/config",
		},
		{
			name:           "macOS",
			goos:           "darwin",
			subPath:        []string{"cache"},
			expectedBase:   "Library/Application Support/Sitepanda/cache",
		},
		{
			name:        "Unsupported OS (windows)",
			goos:        "windows",
			expectError: true,
		},
		{
			name:            "No subPath (Linux with XDG_DATA_HOME)",
			goos:            "linux",
			xdgDataHomeVal:  "/tmp/data_no_subpath",
			subPath:         nil,
			expectedFullDir: "/tmp/data_no_subpath/sitepanda",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS != tt.goos && tt.goos != "windows" {
				t.Skipf("Skipping test %s on %s; test is for %s", tt.name, runtime.GOOS, tt.goos)
			}
			if tt.goos == "windows" && runtime.GOOS != "windows" {
				// This case is to test the error path for unsupported OS.
				// We can't change runtime.GOOS, so we rely on the default case in getAppSubdirectory.
				// If the current OS is supported, this specific test for "windows" error won't run the error path.
				// The function should error out if runtime.GOOS is actually "windows".
				// To truly test this, one would run tests on a Windows machine or mock runtime.GOOS (which is hard).
				// For now, if we are on a supported OS, we expect no error from this specific "windows" test case.
				// If we are on an *actual* unsupported OS, then tt.expectError should be true.
				if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
					if tt.expectError {
						t.Logf("Skipping explicit 'unsupported OS' error check for test case '%s' on supported OS '%s'", tt.name, runtime.GOOS)
						return
					}
				}
			}

			fakeHome := t.TempDir()
			cleanupHomeMock := mockUserHomeDir(t, fakeHome)
			defer cleanupHomeMock()

			if tt.goos == "linux" {
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

			path, err := getAppSubdirectory(tt.subPath...)

			if (err != nil) != tt.expectError {
				t.Fatalf("getAppSubdirectory() error = %v, wantErr %v. Path: %s", err, tt.expectError, path)
			}

			if !tt.expectError {
				if path == "" {
					t.Errorf("getAppSubdirectory() returned empty path")
				}

				var expectedPath string
				if tt.expectedFullDir != "" {
					expectedPath = tt.expectedFullDir
				} else if tt.goos == "linux" && tt.xdgDataHomeVal != "" && tt.useRelativeXDG {
					expectedPath = filepath.Join(fakeHome, tt.expectedBase)
				} else {
					expectedPath = filepath.Join(fakeHome, tt.expectedBase)
				}

				if path != expectedPath {
					t.Errorf("getAppSubdirectory() path = %q, want %q", path, expectedPath)
				}

				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					t.Errorf("getAppSubdirectory() did not create directory: %s", path)
				}
				if strings.HasPrefix(path, fakeHome) || (tt.expectedFullDir != "" && strings.HasPrefix(path, "/tmp")) {
					var baseAppDir string
					if tt.goos == "linux" {
						baseAppDir = "sitepanda"
					} else if tt.goos == "darwin" {
						baseAppDir = "Sitepanda"
					}
					if baseAppDir != "" {
						parts := strings.Split(path, baseAppDir)
						if len(parts) > 0 {
							_ = os.RemoveAll(filepath.Join(parts[0], baseAppDir))
						}
					}
				}
			}
		})
	}
}

func TestGetLightpandaExecutablePath(t *testing.T) {
	fakeHome := t.TempDir()
	cleanupHomeMock := mockUserHomeDir(t, fakeHome)
	defer cleanupHomeMock()

	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDGDataHome)

	path, err := getLightpandaExecutablePath()

	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		if err == nil {
			t.Fatalf("getLightpandaExecutablePath() expected error on unsupported OS %s, but got nil", runtime.GOOS)
		}
		if !strings.Contains(err.Error(), "unsupported OS") {
			t.Errorf("getLightpandaExecutablePath() error = %v, want error containing 'unsupported OS'", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("getLightpandaExecutablePath() error = %v, want nil on supported OS %s", err, runtime.GOOS)
	}

	var expectedDir string
	var appName string
	if runtime.GOOS == "linux" {
		appName = "sitepanda"
		expectedDir = filepath.Join(fakeHome, ".local", "share", appName, "bin")
	} else if runtime.GOOS == "darwin" {
		appName = "Sitepanda" // Corrected to uppercase 'S' for macOS expectation
		expectedDir = filepath.Join(fakeHome, "Library", "Application Support", appName, "bin")
	}

	expectedPath := filepath.Join(expectedDir, lightpandaExecutableName)
	if path != expectedPath {
		t.Errorf("getLightpandaExecutablePath() = %q, want %q", path, expectedPath)
	}

	if _, statErr := os.Stat(filepath.Dir(path)); os.IsNotExist(statErr) {
		t.Errorf("getLightpandaExecutablePath() did not create directory: %s", filepath.Dir(path))
	}
	if runtime.GOOS == "linux" {
		os.RemoveAll(filepath.Join(fakeHome, ".local"))
	} else if runtime.GOOS == "darwin" {
		os.RemoveAll(filepath.Join(fakeHome, "Library"))
	}
}
