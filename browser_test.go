package main

import (
	"errors" // For mockPathError
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep a reference to the original function for GetBrowserExecutablePath
var originalGetBrowserExecutablePath = GetBrowserExecutablePath

// mockGetBrowserExecutablePathHelper mocks GetBrowserExecutablePath for testing.
func mockGetBrowserExecutablePathHelper(t *testing.T, browserName string, expectedBaseDir string, path string, err error) func() {
	t.Helper()
	GetBrowserExecutablePath = func(bName string, baseDir ...string) (string, error) {
		if bName != browserName {
			// If the test is specific to a browser, unexpected calls for other browsers might indicate an issue.
			// For simplicity here, we just pass through to original if browser name doesn't match,
			// or one could fail t.Fatalf("mockGetBrowserExecutablePathHelper called with unexpected browser: %s", bName)
			return originalGetBrowserExecutablePath(bName, baseDir...)
		}
		if len(baseDir) > 0 && baseDir[0] != expectedBaseDir && expectedBaseDir != "" { // expectedBaseDir can be "" if not applicable
			t.Errorf("mockGetBrowserExecutablePathHelper expected baseDir %s, got %s", expectedBaseDir, baseDir[0])
		}
		return path, err
	}
	return func() {
		GetBrowserExecutablePath = originalGetBrowserExecutablePath // Restore original
	}
}

func TestPrepareBrowser(t *testing.T) { // Renamed from TestPrepareLightpanda
	tests := []struct {
		name                    string
		browserName             string // "lightpanda" or "chromium"
		mockPath                string // Path to mock for GetBrowserExecutablePath
		mockPathError           error  // Error to return from mock
		mockBaseDirForChromium  string // Expected base directory for Chromium
		setupFileAtPath         bool   // Create a dummy file at mockPath
		setupDirAtPath          bool   // Create a dummy directory at mockPath
		expectError             bool
		expectedErrorMsgSubstr  string
		expectedLogSubstr       string
	}{
		// Lightpanda cases
		{
			name:                   "Lightpanda success - executable exists",
			browserName:            "lightpanda",
			mockPath:               "temppath/lightpanda_exec/lightpanda", // This path will be joined with tempDir
			setupFileAtPath:        true,
			expectError:            false,
			expectedLogSubstr:      "Using lightpanda binary from:",
		},
		{
			name:                   "Lightpanda failure - GetBrowserExecutablePath returns error",
			browserName:            "lightpanda",
			mockPathError:          os.ErrPermission,
			expectError:            true,
			expectedErrorMsgSubstr: "could not determine lightpanda executable path",
		},
		{
			name:                   "Lightpanda failure - executable not found",
			browserName:            "lightpanda",
			mockPath:               "temppath/nonexistent/lightpanda",
			setupFileAtPath:        false,
			expectError:            true,
			expectedErrorMsgSubstr: "lightpanda executable not found at",
		},
		{
			name:                   "Lightpanda failure - path is a directory",
			browserName:            "lightpanda",
			mockPath:               "temppath/lightpanda_as_dir",
			setupDirAtPath:         true,
			expectError:            true,
			expectedErrorMsgSubstr: "expected lightpanda executable at",
		},
		// Chromium cases
		{
			name:                   "Chromium success - Playwright handles it, GetBrowserExecutablePath hint returns error",
			browserName:            "chromium",
			mockBaseDirForChromium: "temp_playwright_driver_dir_chromium_ok", // Specific base dir for this test
			mockPath:               "temp_playwright_driver_dir_chromium_ok/ms-playwright/version/chromium-blah/chrome", // Example path from GetBrowserExecutablePath
			mockPathError:          errors.New("path hint for Chromium is unreliable"), // Simulate the error GetBrowserExecutablePath returns for Chromium hint
			expectError:            false, // prepareBrowser for chromium should still succeed
			expectedLogSubstr:      "Note: Could not determine a hint for Chromium executable path",
		},
		{
			name:                   "Chromium success - GetBrowserExecutablePath for hint returns different error (still ok)",
			browserName:            "chromium",
			mockBaseDirForChromium: "temp_playwright_driver_dir_chromium_err_hint",
			mockPathError:          errors.New("another error for chromium path hint"),
			expectError:            false,
			expectedLogSubstr:      "Note: Could not determine a hint for Chromium executable path",
		},
		{
			name:                   "Chromium success - GetBrowserExecutablePath for hint returns no error (prepareBrowser logs default message)",
			browserName:            "chromium",
			mockBaseDirForChromium: "temp_playwright_driver_dir_chromium_no_err_hint",
			mockPath:               "some/chromium/path/hint", // Path that doesn't cause GetBrowserExecutablePath to error
			mockPathError:          nil,                       // Simulate GetBrowserExecutablePath not erroring for Chromium
			expectError:            false,
			expectedLogSubstr:      "For Chromium, Playwright is expected to find the executable", // This log appears if GetBrowserExecutablePath doesn't error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			var actualMockPath string
			if tt.mockPath != "" {
				actualMockPath = filepath.Join(tempDir, tt.mockPath)
				if tt.setupFileAtPath || tt.setupDirAtPath {
					if err := os.MkdirAll(filepath.Dir(actualMockPath), 0755); err != nil {
						t.Fatalf("Failed to create parent dir for actualMockPath %s: %v", actualMockPath, err)
					}
				}
			} else if tt.browserName == "lightpanda" {
				actualMockPath = filepath.Join(tempDir, "dummy_lightpanda_path_for_error_case")
			}
			// For Chromium, actualMockPath might be set based on mockPath if provided,
			// but prepareBrowser doesn't os.Stat it, so setupFileAtPath/setupDirAtPath are less relevant for Chromium success cases.

			cleanupMock := mockGetBrowserExecutablePathHelper(t, tt.browserName, tt.mockBaseDirForChromium, actualMockPath, tt.mockPathError)
			defer cleanupMock()

			if tt.setupFileAtPath {
				err := os.WriteFile(actualMockPath, []byte("dummy_executable_content"), 0755)
				if err != nil {
					t.Fatalf("Failed to create dummy executable file at %s: %v", actualMockPath, err)
				}
			}
			if tt.setupDirAtPath {
				err := os.MkdirAll(actualMockPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create dummy directory at %s: %v", actualMockPath, err)
				}
			}

			var logBuf strings.Builder
			originalLoggerOutput := logger.Writer()
			logger.SetOutput(&logBuf)
			defer logger.SetOutput(originalLoggerOutput)

			baseDirArg := ""
			if tt.browserName == "chromium" {
				baseDirArg = tt.mockBaseDirForChromium
			}
			execPath, cleanupFunc, err := prepareBrowser(tt.browserName, baseDirArg)

			if (err != nil) != tt.expectError {
				t.Fatalf("prepareBrowser() error = %v, wantErr %v. Log: %s", err, tt.expectError, logBuf.String())
			}

			if tt.expectError {
				if tt.expectedErrorMsgSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.expectedErrorMsgSubstr)) {
					t.Errorf("prepareBrowser() error = %v, want error containing %q. Log: %s", err, tt.expectedErrorMsgSubstr, logBuf.String())
				}
			} else {
				if tt.browserName == "lightpanda" {
					if execPath == "" {
						t.Errorf("prepareBrowser() expected non-empty executablePath on success for Lightpanda")
					}
					if execPath != actualMockPath {
						t.Errorf("prepareBrowser() execPath = %q, want %q for Lightpanda", execPath, actualMockPath)
					}
				} else if tt.browserName == "chromium" {
					if execPath != "" { // For Chromium, prepareBrowser returns an empty path
						t.Errorf("prepareBrowser() expected empty executablePath on success for Chromium, got %s", execPath)
					}
				}

				if cleanupFunc == nil {
					t.Errorf("prepareBrowser() expected non-nil cleanupFunc")
				}
				cleanupFunc()

				if tt.expectedLogSubstr != "" && !strings.Contains(logBuf.String(), tt.expectedLogSubstr) {
					t.Errorf("prepareBrowser() log output = %q, want to contain %q", logBuf.String(), tt.expectedLogSubstr)
				}
			}
		})
	}
}
