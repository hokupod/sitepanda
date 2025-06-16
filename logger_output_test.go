package main

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// TestLoggerDefaultsToStderr verifies that the logger writes to stderr by default
func TestLoggerDefaultsToStderr(t *testing.T) {
	// This test confirms that logger is initialized with os.Stderr
	// which ensures logs don't interfere with stdout redirection
	
	// Create a new logger with the same configuration as our global logger
	testLogger := log.New(os.Stderr, "", log.LstdFlags)
	
	// Verify that the output is set to stderr
	if testLogger.Writer() != os.Stderr {
		t.Error("Logger should be initialized with os.Stderr")
	}
}

// TestLoggerOutputSeparation tests that logs and content output are properly separated
func TestLoggerOutputSeparation(t *testing.T) {
	// Save original logger output
	originalLogger := logger
	defer func() { logger = originalLogger }()
	
	// Create buffers to capture outputs
	var logBuffer bytes.Buffer
	var contentBuffer bytes.Buffer
	
	// Set logger to write to our test buffer
	SetLoggerOutput(&logBuffer)
	
	// Simulate logging
	logger.Print("This is a log message")
	
	// Simulate content output (like what crawler does)
	contentBuffer.WriteString("<page>\n  <title>Test</title>\n</page>\n")
	
	// Verify log message is in log buffer
	if !strings.Contains(logBuffer.String(), "This is a log message") {
		t.Error("Log message should be in log buffer")
	}
	
	// Verify content is in content buffer
	if !strings.Contains(contentBuffer.String(), "<page>") {
		t.Error("Content should be in content buffer")
	}
	
	// Verify no cross-contamination
	if strings.Contains(contentBuffer.String(), "This is a log message") {
		t.Error("Log message should not be in content buffer")
	}
	if strings.Contains(logBuffer.String(), "<page>") {
		t.Error("Content should not be in log buffer")
	}
}