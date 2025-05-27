package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep a reference to the original function
var originalGetLightpandaExecutablePath = getLightpandaExecutablePath

func mockGetLightpandaExecutablePathHelper(t *testing.T, path string, err error) func() {
	t.Helper()
	getLightpandaExecutablePath = func() (string, error) {
		return path, err
	}
	return func() {
		getLightpandaExecutablePath = originalGetLightpandaExecutablePath // Restore original
	}
}

func TestPrepareLightpanda(t *testing.T) {
	tests := []struct {
		name              string
		mockPath          string
		mockPathError     error
		setupFileAtPath   bool
		setupDirAtPath    bool
		expectError       bool
		expectedErrorMsg  string
		expectedLogSubstr string
	}{
		{
			name:              "success - lightpanda executable exists",
			mockPath:          "temppath/lightpanda",
			setupFileAtPath:   true,
			expectError:       false,
			expectedLogSubstr: "Using Lightpanda binary from:",
		},
		{
			name:             "failure - getLightpandaExecutablePath returns error",
			mockPathError:    os.ErrPermission,
			expectError:      true,
			expectedErrorMsg: "could not determine Lightpanda executable path",
		},
		{
			name:             "failure - lightpanda executable not found",
			mockPath:         "temppath/nonexistent/lightpanda",
			setupFileAtPath:  false,
			expectError:      true,
			expectedErrorMsg: "Please run 'sitepanda init'",
		},
		{
			name:             "failure - path is a directory",
			mockPath:         "temppath/lightpanda_as_dir",
			setupDirAtPath:   true,
			expectError:      true,
			expectedErrorMsg: "expected Lightpanda executable at",
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
			} else {
				actualMockPath = filepath.Join(tempDir, "dummy_path_for_error_case")
			}

			cleanupMock := mockGetLightpandaExecutablePathHelper(t, actualMockPath, tt.mockPathError)
			defer cleanupMock()

			if tt.setupFileAtPath {
				err := os.WriteFile(actualMockPath, []byte("dummy_executable_content"), 0755)
				if err != nil {
					t.Fatalf("Failed to create dummy executable file at %s: %v", actualMockPath, err)
				}
				// No defer os.Remove here, t.TempDir() handles cleanup of its contents
			}

			if tt.setupDirAtPath {
				err := os.MkdirAll(actualMockPath, 0755)
				if err != nil {
					t.Fatalf("Failed to create dummy directory at %s: %v", actualMockPath, err)
				}
				// No defer os.RemoveAll here, t.TempDir() handles cleanup
			}

			var logBuf strings.Builder
			originalLoggerOutput := logger.Writer()
			logger.SetOutput(&logBuf)
			defer logger.SetOutput(originalLoggerOutput)

			execPath, cleanupFunc, err := prepareLightpanda()

			if (err != nil) != tt.expectError {
				t.Fatalf("prepareLightpanda() error = %v, wantErr %v", err, tt.expectError)
			}

			if tt.expectError {
				if tt.expectedErrorMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.expectedErrorMsg)) {
					t.Errorf("prepareLightpanda() error = %v, want error containing %q", err, tt.expectedErrorMsg)
				}
			} else {
				if execPath == "" {
					t.Errorf("prepareLightpanda() expected non-empty executablePath on success")
				}
				if execPath != actualMockPath {
					t.Errorf("prepareLightpanda() execPath = %q, want %q", execPath, actualMockPath)
				}
				if cleanupFunc == nil {
					t.Errorf("prepareLightpanda() expected non-nil cleanupFunc")
				}
				cleanupFunc() // Call to ensure no panic

				if tt.expectedLogSubstr != "" && !strings.Contains(logBuf.String(), tt.expectedLogSubstr) {
					t.Errorf("prepareLightpanda() log output = %q, want to contain %q", logBuf.String(), tt.expectedLogSubstr)
				}
			}
		})
	}
}
