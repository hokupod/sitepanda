package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkill(t *testing.T) {
	// Create a temporary directory for tests
	tempDir, err := os.MkdirTemp("", "sitepanda-skill-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockHomeDir := filepath.Join(tempDir, "home")
	if err := os.Mkdir(mockHomeDir, 0755); err != nil {
		t.Fatalf("Failed to create mock home dir: %v", err)
	}

	getMockHome := func() (string, error) {
		return mockHomeDir, nil
	}

	tests := []struct {
		name           string
		input          string
		env            map[string]string
		expectedOutput []string
		expectedPath   string
		expectedError  string
		setup          func()
	}{
		{
			name:  "Install for OpenAI Codex (Default)",
			input: "1\n",
			env:   nil,
			expectedOutput: []string{
				"Which AI coding tool do you want to install",
				"Sitepanda skill installed for OpenAI Codex",
			},
			expectedPath: filepath.Join(mockHomeDir, ".codex", "skills", "sitepanda"),
		},
		{
			name:  "Install for OpenAI Codex (With Valid CODEX_HOME)",
			input: "1\n",
			env: map[string]string{
				"CODEX_HOME": filepath.Join(tempDir, "custom_codex"),
			},
			expectedOutput: []string{
				"Sitepanda skill installed for OpenAI Codex",
			},
			expectedPath: filepath.Join(tempDir, "custom_codex", "skills", "sitepanda"),
		},
		{
			name:  "Install for OpenAI Codex (With Invalid CODEX_HOME - System Root)",
			input: "1\n",
			env: map[string]string{
				"CODEX_HOME": "/",
			},
			expectedError: "CODEX_HOME points to a system directory: /",
		},
		{
			name:  "Install for OpenAI Codex (With Invalid CODEX_HOME - /etc)",
			input: "1\n",
			env: map[string]string{
				"CODEX_HOME": "/etc",
			},
			expectedError: "CODEX_HOME points to a system directory: /etc",
		},
		{
			name:  "Install for Claude Code",
			input: "2\n",
			env:   nil,
			expectedOutput: []string{
				"Sitepanda skill installed for Claude Code",
			},
			expectedPath: filepath.Join(mockHomeDir, ".claude", "skills", "sitepanda"),
		},
		{
			name:  "Install with Overwrite (Yes)",
			input: "1\ny\n",
			env:   nil,
			setup: func() {
				path := filepath.Join(mockHomeDir, ".codex", "skills", "sitepanda")
				os.MkdirAll(path, 0755)
			},
			expectedOutput: []string{
				"Do you want to overwrite SKILL.md in this directory?",
				"Sitepanda skill installed for OpenAI Codex",
			},
			expectedPath: filepath.Join(mockHomeDir, ".codex", "skills", "sitepanda"),
		},
		{
			name:  "Install to Custom Path",
			input: "3\n" + filepath.Join(tempDir, "custom", "path") + "\n",
			env:   nil,
			expectedOutput: []string{
				"Enter the installation directory path:",
				"Sitepanda skill installed for Custom Tool",
			},
			expectedPath: filepath.Join(tempDir, "custom", "path"),
		},
		{
			name:  "Install to Custom Path (With Tilde Expansion)",
			input: "3\n~/custom/tilde/path\n",
			env:   nil,
			expectedOutput: []string{
				"Sitepanda skill installed for Custom Tool",
			},
			expectedPath: filepath.Join(mockHomeDir, "custom", "tilde", "path"),
		},
		{
			name:  "Install to Custom Path (Standalone Tilde)",
			input: "3\n~\n",
			env:   nil,
			expectedOutput: []string{
				"Sitepanda skill installed for Custom Tool",
			},
			expectedPath: mockHomeDir,
		},
		{
			name:  "Install to Custom Path (Invalid - System Directory)",
			input: "3\n/etc\n",
			env:   nil,
			expectedError: "installation path points to a system directory: /etc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous runs if necessary (though tempDir helps isolate)
			// But for overwrite test, we need to setup
			if tt.setup != nil {
				tt.setup()
			} else {
				// Ensure clean state for expected path if it was created by previous test
				if tt.expectedPath != "" {
					os.RemoveAll(tt.expectedPath)
				}
			}

			mockEnv := func(key string) string {
				if val, ok := tt.env[key]; ok {
					return val
				}
				return ""
			}

			input := bytes.NewBufferString(tt.input)
			output := new(bytes.Buffer)

			err := installSkill(input, output, mockEnv, getMockHome)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error to contain %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				outputStr := output.String()
				for _, expected := range tt.expectedOutput {
					if !strings.Contains(outputStr, expected) {
						t.Errorf("Expected output to contain %q, got:\n%s", expected, outputStr)
					}
				}

				if tt.expectedPath != "" {
					if _, err := os.Stat(tt.expectedPath); os.IsNotExist(err) {
						t.Errorf("Expected directory %s to exist, but it does not exist", tt.expectedPath)
					}
					skillFile := filepath.Join(tt.expectedPath, "SKILL.md")
					if _, err := os.Stat(skillFile); os.IsNotExist(err) {
						t.Errorf("Expected SKILL.md to exist at %s", skillFile)
					}
				}
			}
		})
	}
}

func TestInstallSkill_OverwriteCancel(t *testing.T) {
	// Separate test for overwrite cancel to handle setup/teardown cleanly
	tempDir, err := os.MkdirTemp("", "sitepanda-skill-cancel-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockHomeDir := filepath.Join(tempDir, "home")
	installPath := filepath.Join(mockHomeDir, ".codex", "skills", "sitepanda")
	if err := os.MkdirAll(installPath, 0755); err != nil {
		t.Fatalf("Failed to create install dir: %v", err)
	}

	getMockHome := func() (string, error) {
		return mockHomeDir, nil
	}
	mockEnv := func(key string) string { return "" }

	// Input: Select Codex (1), then say No (n) to overwrite
	input := bytes.NewBufferString("1\nn\n")
	output := new(bytes.Buffer)

	err = installSkill(input, output, mockEnv, getMockHome)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	outputStr := output.String()
	expectedMsg := "Do you want to overwrite SKILL.md in this directory? (y/N):"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected output to contain overwrite prompt %q, got:\n%s", expectedMsg, outputStr)
	}
	if !strings.Contains(outputStr, "Installation cancelled") {
		t.Errorf("Expected output to contain 'Installation cancelled', got:\n%s", outputStr)
	}
}
