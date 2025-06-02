package main

import (
	"flag"
	"io"
	"os"
	"testing"
)

func setupTestFlagsAndParse(t *testing.T, args []string) (calculatedDefaultBrowser string, finalBrowserName string, finalWaitForNetworkIdle bool, parseErr error) {
	t.Helper()

	originalOsArgs := os.Args
	currentExe, _ := os.Executable()
	os.Args = append([]string{currentExe}, args...)
	defer func() { os.Args = originalOsArgs }()

	expectedDefaultBrowser := "lightpanda"
	envBrowser := os.Getenv("SITEPANDA_BROWSER")
	if envBrowser != "" {
		if envBrowser == "lightpanda" || envBrowser == "chromium" {
			expectedDefaultBrowser = envBrowser
		}
	}
	calculatedDefaultBrowser = expectedDefaultBrowser

	testFlagSet := flag.NewFlagSet("testargs", flag.ContinueOnError)
	testFlagSet.SetOutput(io.Discard)

	var capturedBrowserName string
	testFlagSet.StringVar(&capturedBrowserName, "browser", expectedDefaultBrowser, "Browser to use")
	testFlagSet.StringVar(&capturedBrowserName, "b", expectedDefaultBrowser, "Browser to use (shorthand)")

	var dummyInt int
	var dummyString string
	var dummyStringSlice stringSlice
	var dummyBool bool
	var capturedWaitForNetworkIdle bool
	testFlagSet.IntVar(&dummyInt, "limit", 0, "dummy limit")
	testFlagSet.StringVar(&dummyString, "outfile", "", "dummy outfile")
	testFlagSet.StringVar(&dummyString, "o", "", "dummy o")
	testFlagSet.Var(&dummyStringSlice, "match", "dummy match")
	testFlagSet.Var(&dummyStringSlice, "m", "dummy m")
	testFlagSet.StringVar(&dummyString, "content-selector", "", "dummy selector")
	testFlagSet.BoolVar(&dummyBool, "silent", false, "dummy silent")
	testFlagSet.BoolVar(&dummyBool, "version", false, "dummy version")
	testFlagSet.BoolVar(&capturedWaitForNetworkIdle, "wait-for-network-idle", false, "dummy wni long")
	testFlagSet.BoolVar(&capturedWaitForNetworkIdle, "wni", false, "dummy wni short")

	parseErr = testFlagSet.Parse(os.Args[1:])
	if parseErr != nil {
		return calculatedDefaultBrowser, "", false, parseErr
	}
	finalBrowserName = capturedBrowserName
	finalWaitForNetworkIdle = capturedWaitForNetworkIdle
	return calculatedDefaultBrowser, finalBrowserName, finalWaitForNetworkIdle, nil
}

func TestBrowserSelectionLogic(t *testing.T) {
	originalEnvBrowser, originalEnvBrowserExists := os.LookupEnv("SITEPANDA_BROWSER")
	defer func() {
		if originalEnvBrowserExists {
			os.Setenv("SITEPANDA_BROWSER", originalEnvBrowser)
		} else {
			os.Unsetenv("SITEPANDA_BROWSER")
		}
	}()

	tests := []struct {
		name                       string
		envVarValue                string
		cliArgs                    []string
		expectedDefaultBrowserVal  string
		expectedFinalBrowserName   string
		expectedWaitForNetworkIdle bool
		expectParseError           bool
	}{
		{
			name:                       "default (no env, no browser args)",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env lightpanda, no browser args",
			envVarValue:                "lightpanda",
			cliArgs:                    []string{"http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env chromium, no browser args",
			envVarValue:                "chromium",
			cliArgs:                    []string{"http://example.com"},
			expectedDefaultBrowserVal:  "chromium",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env invalid, no browser args",
			envVarValue:                "firefox",
			cliArgs:                    []string{"http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "arg -b chromium, no env",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"-b", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "arg --browser chromium, no env",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"--browser", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env lightpanda, arg -b chromium",
			envVarValue:                "lightpanda",
			cliArgs:                    []string{"-b", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env chromium, arg --browser lightpanda",
			envVarValue:                "chromium",
			cliArgs:                    []string{"--browser", "lightpanda", "http://example.com"},
			expectedDefaultBrowserVal:  "chromium",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "env invalid, arg -b chromium",
			envVarValue:                "firefox",
			cliArgs:                    []string{"-b", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: false,
		},
		{
			name:                       "arg -wni, no env",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"-wni", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: true,
		},
		{
			name:                       "arg --wait-for-network-idle, no env",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"--wait-for-network-idle", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "lightpanda",
			expectedWaitForNetworkIdle: true,
		},
		{
			name:                       "arg -wni and -b chromium, no env",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"-wni", "-b", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: true,
		},
		{
			name:                       "arg --wait-for-network-idle and --browser chromium, env lightpanda",
			envVarValue:                "lightpanda",
			cliArgs:                    []string{"--wait-for-network-idle", "--browser", "chromium", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedFinalBrowserName:   "chromium",
			expectedWaitForNetworkIdle: true,
		},
		{
			name:                       "unknown flag causes parse error",
			envVarValue:                "UNSET",
			cliArgs:                    []string{"--unknown-flag", "http://example.com"},
			expectedDefaultBrowserVal:  "lightpanda",
			expectedWaitForNetworkIdle: false,
			expectParseError:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarValue == "UNSET" {
				os.Unsetenv("SITEPANDA_BROWSER")
			} else {
				os.Setenv("SITEPANDA_BROWSER", tt.envVarValue)
			}

			calculatedDefault, finalBrowser, finalWNI, err := setupTestFlagsAndParse(t, tt.cliArgs)

			if tt.expectParseError {
				if err == nil {
					t.Errorf("expected a parse error, but got nil. Final browser: %s, final WNI: %t", finalBrowser, finalWNI)
				}
				return
			}
			if err != nil {
				t.Fatalf("setupTestFlagsAndParse failed: %v", err)
			}

			if calculatedDefault != tt.expectedDefaultBrowserVal {
				t.Errorf("Calculated defaultBrowser mismatch: got %q, want %q (env: %q)",
					calculatedDefault, tt.expectedDefaultBrowserVal, tt.envVarValue)
			}

			if finalBrowser != tt.expectedFinalBrowserName {
				t.Errorf("Final browserName mismatch: got %q, want %q (env: %q, args: %v)",
					finalBrowser, tt.expectedFinalBrowserName, tt.envVarValue, tt.cliArgs)
			}

			if finalWNI != tt.expectedWaitForNetworkIdle {
				t.Errorf("Final waitForNetworkIdle mismatch: got %t, want %t (env: %q, args: %v)",
					finalWNI, tt.expectedWaitForNetworkIdle, tt.envVarValue, tt.cliArgs)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than maxLen",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to maxLen",
			s:      "hello world",
			maxLen: 11,
			want:   "hello world",
		},
		{
			name:   "string longer than maxLen",
			s:      "hello world example",
			maxLen: 11,
			want:   "hello world",
		},
		{
			name:   "maxLen is 0",
			s:      "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "empty string",
			s:      "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "maxLen is negative",
			s:      "hello",
			maxLen: -1,
			want:   "",
		},
		{
			name:   "multibyte characters, shorter",
			s:      "こんにちは",
			maxLen: 10,
			want:   "こんにちは",
		},
		{
			name:   "multibyte characters, truncate",
			s:      "こんにちは世界",
			maxLen: 5,
			want:   "こんにちは",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = stringSlice{}

			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}
