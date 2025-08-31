package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello world", 5, "hello"},
		{"hello", 10, "hello"},
		{"", 5, ""},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"🚀🎉✨", 2, "🚀🎉"},
		{"test", 4, "test"},
		{"a", 1, "a"},
		{"ab", 1, "a"},
		{"multi\nline\ntext", 8, "multi\nli"},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+string(rune(tt.maxLen)), func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestSetLoggerOutput(t *testing.T) {
	// Save original logger
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Test setting logger output
	var testOutput bytes.Buffer
	SetLoggerOutput(&testOutput)

	// Write a test message
	logger.Print("test message for logger output")

	// Check that the message was written to our test output
	output := testOutput.String()
	if !strings.Contains(output, "test message for logger output") {
		t.Errorf("Expected test output to contain 'test message for logger output', got %q", output)
	}
}

func TestConstants(t *testing.T) {
	t.Run("Version constant", func(t *testing.T) {
		if Version == "" {
			t.Error("Version constant should not be empty")
		}
		if Version != "0.3.0" {
			t.Errorf("Expected Version to be '0.3.0', got %q", Version)
		}
	})

	t.Run("LightpandaNightlyVersion constant", func(t *testing.T) {
		if LightpandaNightlyVersion == "" {
			t.Error("LightpandaNightlyVersion constant should not be empty")
		}
		if LightpandaNightlyVersion != "nightly" {
			t.Errorf("Expected LightpandaNightlyVersion to be 'nightly', got %q", LightpandaNightlyVersion)
		}
	})
}

func TestLoggerExists(t *testing.T) {
	// Test that logger is not nil and functional
	if logger == nil {
		t.Fatal("logger should not be nil")
	}

	// Test that logger can write without panicking
	var buf bytes.Buffer
	originalLogger := logger
	defer func() { logger = originalLogger }()

	SetLoggerOutput(&buf)

	// This should not panic
	logger.Print("test")

	if buf.Len() == 0 {
		t.Error("Expected logger to write some output")
	}
}
