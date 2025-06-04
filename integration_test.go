package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Integration tests that run the actual binary
// These tests require the binary to be built first

func TestCLIIntegration(t *testing.T) {
	// Build the binary for testing
	binaryPath := filepath.Join(os.TempDir(), "sitepanda-test")
	if err := exec.Command("go", "build", "-o", binaryPath, ".").Run(); err != nil {
		t.Fatalf("Failed to build binary for testing: %v", err)
	}
	defer os.Remove(binaryPath)

	tests := []struct {
		name           string
		args           []string
		expectError    bool
		expectedOutput string
		skipReason     string
	}{
		{
			name:           "Help command",
			args:           []string{"--help"},
			expectError:    false,
			expectedOutput: "Sitepanda is a command-line interface",
		},
		{
			name:           "Version command",
			args:           []string{"--version"},
			expectError:    false,
			expectedOutput: "0.1.0",
		},
		{
			name:           "Init help",
			args:           []string{"init", "--help"},
			expectError:    false,
			expectedOutput: "Download and install",
		},
		{
			name:           "Scrape help",
			args:           []string{"scrape", "--help"},
			expectError:    false,
			expectedOutput: "Scrape websites using",
		},
		{
			name:        "Init without actual installation",
			args:        []string{"init", "--help"}, // Use help to avoid actual installation
			expectError: false,
			skipReason:  "Actual init requires network and takes time",
		},
		{
			name:        "Scrape without URL",
			args:        []string{"scrape"},
			expectError: true,
			skipReason:  "Requires browser setup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but command succeeded. Output: %s", output)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			if tt.expectedOutput != "" && !strings.Contains(string(output), tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, string(output))
			}
		})
	}
}

func TestCLIBrowserFlags(t *testing.T) {
	// Test that browser flags are properly passed through
	binaryPath := filepath.Join(os.TempDir(), "sitepanda-test")
	if err := exec.Command("go", "build", "-o", binaryPath, ".").Run(); err != nil {
		t.Fatalf("Failed to build binary for testing: %v", err)
	}
	defer os.Remove(binaryPath)

	tests := []struct {
		name        string
		args        []string
		expectError bool
		skipReason  string
	}{
		{
			name:        "Global browser flag with init",
			args:        []string{"--browser", "chromium", "init", "--help"},
			expectError: false,
		},
		{
			name:        "Global browser flag with scrape",
			args:        []string{"--browser", "lightpanda", "scrape", "--help"},
			expectError: false,
		},
		{
			name:        "Short browser flag",
			args:        []string{"-b", "chromium", "init", "--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but command succeeded. Output: %s", output)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}
		})
	}
}

// Test environment variable handling
func TestEnvironmentVariables(t *testing.T) {
	binaryPath := filepath.Join(os.TempDir(), "sitepanda-test")
	if err := exec.Command("go", "build", "-o", binaryPath, ".").Run(); err != nil {
		t.Fatalf("Failed to build binary for testing: %v", err)
	}
	defer os.Remove(binaryPath)

	// Save original environment
	originalEnv := os.Getenv("SITEPANDA_BROWSER")
	defer func() {
		if originalEnv != "" {
			os.Setenv("SITEPANDA_BROWSER", originalEnv)
		} else {
			os.Unsetenv("SITEPANDA_BROWSER")
		}
	}()

	tests := []struct {
		name       string
		envValue   string
		args       []string
		skipReason string
	}{
		{
			name:     "Environment variable chromium",
			envValue: "chromium",
			args:     []string{"init", "--help"},
		},
		{
			name:     "Environment variable lightpanda",
			envValue: "lightpanda",
			args:     []string{"init", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			// Set environment variable
			os.Setenv("SITEPANDA_BROWSER", tt.envValue)

			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			// The output should contain help text since we're using --help
			if !strings.Contains(string(output), "Download and install") {
				t.Errorf("Expected help output, got %q", string(output))
			}
		})
	}
}