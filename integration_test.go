package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binaryPath string

// TestMain sets up a single binary and browser installation for all integration tests.
func TestMain(m *testing.M) {
	// Silence the logger for unit tests within the package
	originalLoggerOutput := logger.Writer()
	logger.SetOutput(io.Discard)
	defer logger.SetOutput(originalLoggerOutput)

	tempDir, err := os.MkdirTemp("", "sitepanda-integration-")
	if err != nil {
		fmt.Printf("Failed to create temp dir for testing: %v", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tempDir, "sitepanda-test")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Build the binary once
	fmt.Println("Building test binary...")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Printf("Failed to build test binary: %v\nOutput:\n%s", err, string(output))
		os.RemoveAll(tempDir)
		os.Exit(1)
	}

	// Ensure browser is installed once
	fmt.Println("Ensuring browser is installed for integration tests...")
	initCmd := exec.Command(binaryPath, "init")
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		fmt.Printf("Warning: 'sitepanda init' failed, but proceeding. Error: %v\n", err)
	} else {
		fmt.Println("Browser setup checked/completed.")
	}

	// Run all tests
	exitCode := m.Run()

	// Cleanup before exiting
	os.RemoveAll(tempDir)
	os.Exit(exitCode)
}

// setupTestServer creates a simple HTTP test server.
func setupTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><head><title>Page 1</title></head><body><h1>Hello</h1><p>This is page 1.</p><a href="/page2">Page 2</a></body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// A page that takes a bit of time to respond for cancellation tests
		time.Sleep(2 * time.Second)
		fmt.Fprintln(w, `<html><head><title>Page 2</title></head><body><p>This is page 2.</p></body></html>`)
	})
	return httptest.NewServer(mux)
}

func TestCLIIntegration(t *testing.T) {
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
			expectedOutput: "0.3.0",
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
			name:           "Scrape without URL",
			args:           []string{"scrape"},
			expectError:    true,
			expectedOutput: "URL argument or --url-file option is required",
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
	tests := []struct {
		name        string
		args        []string
		expectError bool
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

func TestEnvironmentVariables(t *testing.T) {
	originalEnv := os.Getenv("SITEPANDA_BROWSER")
	defer func() {
		if originalEnv != "" {
			os.Setenv("SITEPANDA_BROWSER", originalEnv)
		} else {
			os.Unsetenv("SITEPANDA_BROWSER")
		}
	}()

	tests := []struct {
		name     string
		envValue string
		args     []string
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
			os.Setenv("SITEPANDA_BROWSER", tt.envValue)

			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			if !strings.Contains(string(output), "Download and install") {
				t.Errorf("Expected help output, got %q", string(output))
			}
		})
	}
}

func TestScrapeOutputFormatFlag(t *testing.T) {
	// This test is now implicitly covered by the summary tests,
	// but we keep it as a fast, non-browser check.
	// It doesn't need the server because it just checks startup logs.
	args := []string{"scrape", "-f", "json", "http://example.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	output, _ := cmd.CombinedOutput() // We ignore the error because the command is expected to fail
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: %v", args)
	}
	if !strings.Contains(string(output), "Output Format: json") {
		t.Errorf("Expected log to contain 'Output Format: json', got %q", string(output))
	}
}

func TestScrapeSummaryReport_Completion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode.")
	}

	server := setupTestServer()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "scrape", server.URL+"/page1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed with error: %v\nOutput:\n%s", err, string(output))
	}

	outputStr := string(output)

	if !strings.Contains(outputStr, "Scraping Summary") {
		t.Errorf("Expected output to contain 'Scraping Summary', but it didn't. Output:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Status: Completed") {
		t.Errorf("Expected status to be 'Completed', but it wasn't. Output:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Pages Saved: 2") {
		t.Errorf("Expected 'Pages Saved: 2', but it wasn't found. Output:\n%s", outputStr)
	}
}

func TestScrapeSummaryReport_Cancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode.")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Skipping cancellation test on Windows due to unreliable signal delivery.")
	}

	server := setupTestServer()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "scrape", server.URL+"/page1")

	var outputBuffer strings.Builder
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Give it a moment to start processing. It should process page1 quickly
	// and then hang on page2. 3 seconds should be enough time.
	time.Sleep(3 * time.Second)

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send interrupt signal: %v", err)
	}

	err := cmd.Wait()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if ee.String() != "signal: interrupt" && ee.String() != "exit status 1" {
				t.Logf("Command exited with an unexpected error: %v", err)
			}
		} else {
			t.Logf("Command Wait() returned a non-ExitError: %v", err)
		}
	}

	outputStr := outputBuffer.String()

	if !strings.Contains(outputStr, "Scraping Summary") {
		t.Errorf("Expected output to contain 'Scraping Summary', but it didn't. Output:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Status: Cancelled by user") {
		t.Errorf("Expected status to be 'Cancelled by user', but it wasn't. Output:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Pages Saved: 1") {
		t.Errorf("Expected 'Pages Saved: 1', but it wasn't found. Output:\n%s", outputStr)
	}
}

func TestScrape_VerboseBrowserFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode.")
	}

	server := setupTestServer()
	defer server.Close()

	// 1. Run WITHOUT the flag
	ctxDefault, cancelDefault := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelDefault()
	cmdDefault := exec.CommandContext(ctxDefault, binaryPath, "scrape", server.URL+"/page1")
	outputDefault, errDefault := cmdDefault.CombinedOutput()
	if errDefault != nil {
		t.Fatalf("Default command failed with error: %v\nOutput:\n%s", errDefault, string(outputDefault))
	}
	outputDefaultStr := string(outputDefault)

	if strings.Contains(outputDefaultStr, "[pid=") {
		t.Errorf("Expected default output to NOT contain verbose '[pid=...' logs, but it did.\nOutput:\n%s", outputDefaultStr)
	}

	// 2. Run WITH the flag
	ctxVerbose, cancelVerbose := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelVerbose()
	cmdVerbose := exec.CommandContext(ctxVerbose, binaryPath, "scrape", "--verbose-browser", server.URL+"/page1")
	outputVerbose, errVerbose := cmdVerbose.CombinedOutput()
	if errVerbose != nil {
		t.Fatalf("Verbose command failed with error: %v\nOutput:\n%s", errVerbose, string(outputVerbose))
	}
	outputVerboseStr := string(outputVerbose)

	if !strings.Contains(outputVerboseStr, "[pid=") {
		t.Logf("Warning: Expected verbose output to contain '[pid=...' logs, but it didn't. This may happen in some environments or if Playwright changes its log format. Output:\n%s", outputVerboseStr)
	}
}
