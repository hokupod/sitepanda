package main

import (
	"io"
	"log"
	"os"
)

const (
	Version                  = "0.1.3"
	LightpandaNightlyVersion = "nightly"
)

// logger is a global logger instance
var logger = log.New(os.Stderr, "", log.LstdFlags)

// SetLoggerOutput sets the output destination for the logger
func SetLoggerOutput(w io.Writer) {
	logger.SetOutput(w)
}

// truncateString truncates a string to maxLen runes
func truncateString(s string, maxLen int) string {
	if maxLen < 0 {
		maxLen = 0
	}
	if maxLen == 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
