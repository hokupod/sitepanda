package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSkillInstallCmd_Subprocess tests the skill install command logic including os.Exit
func TestSkillInstallCmd_Subprocess(t *testing.T) {
	if os.Getenv("TEST_SKILL_CMD") == "1" {
		// Mock the handler
		mode := os.Getenv("TEST_MODE")
		if mode == "success" {
			SkillInstallHandler = func() error { return nil }
		} else if mode == "error" {
			SkillInstallHandler = func() error { return fmt.Errorf("mock error") }
		} else {
			SkillInstallHandler = nil
		}

		// Execute the command
		skillInstallCmd.Run(skillInstallCmd, []string{})
		return
	}

	tests := []struct {
		name           string
		mode           string
		expectedExit   bool
		expectedStderr string
	}{
		{
			name:           "Success",
			mode:           "success",
			expectedExit:   false,
			expectedStderr: "",
		},
		{
			name:           "Handler Error",
			mode:           "error",
			expectedExit:   true,
			expectedStderr: "Error: mock error",
		},
		{
			name:           "Handler Not Set",
			mode:           "nil",
			expectedExit:   true,
			expectedStderr: "Error: Skill install handler not set.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestSkillInstallCmd_Subprocess")
			cmd.Env = append(os.Environ(), "TEST_SKILL_CMD=1", "TEST_MODE="+tt.mode)

			// Capture output
			var stderr strings.Builder
			cmd.Stderr = &stderr

			err := cmd.Run()

			// Check exit code
			if tt.expectedExit {
				if err == nil {
					t.Error("Expected exit status 1, but command succeeded")
				} else if _, ok := err.(*exec.ExitError); !ok {
					t.Errorf("Expected exit error, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, got error %v", err)
				}
			}

			// Check stderr
			if tt.expectedStderr != "" {
				if !strings.Contains(stderr.String(), tt.expectedStderr) {
					t.Errorf("Expected stderr to contain %q, got %q", tt.expectedStderr, stderr.String())
				}
			}
		})
	}
}
