package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HandleSkillInstall handles the interactive installation of the Sitepanda skill
func HandleSkillInstall() error {
	return installSkill(os.Stdin, os.Stdout, os.Getenv, os.UserHomeDir)
}

// installSkill contains the core logic for installing the skill, separated for testability
func installSkill(input io.Reader, output io.Writer, getEnv func(string) string, getHomeDir func() (string, error)) error {
	reader := bufio.NewReader(input)

	fmt.Fprintln(output, "Which AI coding tool do you want to install the Sitepanda skill for?")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "1) OpenAI Codex")
	fmt.Fprintln(output, "2) Claude Code")
	fmt.Fprintln(output, "3) Other (Custom Path)")
	fmt.Fprintln(output)
	fmt.Fprint(output, "Enter your choice (1, 2 or 3): ")

	choiceStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	choiceStr = strings.TrimSpace(choiceStr)

	var installPath string
	var toolName string

	switch choiceStr {
	case "1":
		toolName = "OpenAI Codex"
		// Check CODEX_HOME environment variable
		if codexHome := getEnv("CODEX_HOME"); codexHome != "" {
			if err := validateSystemPath(codexHome); err != nil {
				return fmt.Errorf("CODEX_HOME %w", err)
			}
			installPath = filepath.Join(codexHome, "skills", "sitepanda")
		} else {
			homeDir, err := getHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			installPath = filepath.Join(homeDir, ".codex", "skills", "sitepanda")
		}
	case "2":
		toolName = "Claude Code"
		// Check CLAUDE_HOME environment variable
		if claudeHome := getEnv("CLAUDE_HOME"); claudeHome != "" {
			if err := validateSystemPath(claudeHome); err != nil {
				return fmt.Errorf("CLAUDE_HOME %w", err)
			}
			installPath = filepath.Join(claudeHome, "skills", "sitepanda")
		} else {
			homeDir, err := getHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			installPath = filepath.Join(homeDir, ".claude", "skills", "sitepanda")
		}
	case "3":
		toolName = "Custom Tool"
		fmt.Fprint(output, "Enter the installation directory path: ")
		customPath, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		customPath = strings.TrimSpace(customPath)
		if customPath == "" {
			return fmt.Errorf("installation directory path cannot be empty")
		}

		// Expand '~' to user home directory if it's the first character
		// Handle both ~/. (normalized to ~ by shell usually, but here we treat raw input)
		// and simple ~
		// Also handle ~/ with both forward and backslash for robustness
		if customPath == "~" || strings.HasPrefix(customPath, "~/") || strings.HasPrefix(customPath, "~\\") {
			homeDir, err := getHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			if customPath == "~" {
				customPath = homeDir
			} else {
				// Remove the ~ and the separator (2 chars) and join with homeDir
				customPath = filepath.Join(homeDir, customPath[2:])
			}
		}

		if err := validateSystemPath(customPath); err != nil {
			return fmt.Errorf("installation path %w", err)
		}
		installPath = customPath
	default:
		return fmt.Errorf("invalid choice: %s", choiceStr)
	}

	// Check if directory already exists
	if _, err := os.Stat(installPath); err == nil {
		fmt.Fprintf(output, "Warning: The directory %s already exists.\n", installPath)
		fmt.Fprint(output, "Do you want to overwrite SKILL.md in this directory? (y/N): ")
		overwriteStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		overwriteStr = strings.TrimSpace(strings.ToLower(overwriteStr))
		if overwriteStr != "y" && overwriteStr != "yes" {
			fmt.Fprintln(output, "Installation cancelled.")
			return nil
		}
	}

	// Create directory
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", installPath, err)
	}

	// Create SKILL.md
	skillFilePath := filepath.Join(installPath, "SKILL.md")
	if err := os.WriteFile(skillFilePath, []byte(skillMdContent), 0644); err != nil {
		return fmt.Errorf("failed to write SKILL.md to %s: %w", skillFilePath, err)
	}

	fmt.Fprintf(output, "✔ Sitepanda skill installed for %s\n", toolName)
	fmt.Fprintf(output, "  Path: %s\n\n", installPath)
	fmt.Fprintf(output, "Restart %s to load the new skill.\n", toolName)

	return nil
}

// validateSystemPath checks if the given path is a system directory
func validateSystemPath(path string) error {
	cleanPath := filepath.Clean(path)

	// List of restricted system directories
	restrictedPaths := []string{
		"/", "/etc", "/usr", "/bin", "/sbin", "/var",
		"/tmp", "/opt", "/boot", "/dev", "/proc", "/sys", "/root",
	}

	for _, restricted := range restrictedPaths {
		if cleanPath == restricted || strings.HasPrefix(cleanPath, restricted+string(os.PathSeparator)) {
			return fmt.Errorf("points to a system directory: %s", cleanPath)
		}
	}

	// Basic Windows system path check
	// Check if path starts with common Windows system paths (case-insensitive)
	lowerPath := strings.ToLower(cleanPath)
	winRestricted := []string{
		"c:\\windows",
		"c:\\program files",
		"c:\\program files (x86)",
	}

	for _, restricted := range winRestricted {
		if lowerPath == restricted || strings.HasPrefix(lowerPath, restricted+"\\") {
			return fmt.Errorf("points to a system directory: %s", cleanPath)
		}
	}

	return nil
}

const skillMdContent = `---
name: sitepanda
description: >
  Scrape websites with a headless browser and extract main readable content as Markdown.
  Use this skill when the user asks to retrieve, analyze, or summarize content from a URL or website.
---

# Sitepanda (Web Scraping Tool)

## Instructions

1. When the user provides a URL or asks for website content, use Sitepanda to scrape the page.
2. By default, use the following command to scrape a single page:

   sitepanda scrape <URL> --silent --limit 1

3. If you need to perform recursive scraping (following links), you **must** ask the user for confirmation before starting, as it may take a long time.
4. Capture the output, which is returned in Markdown format.
5. Read and analyze the extracted content.
6. Respond to the user using only the relevant information from the page.
7. If the content is long, summarize or extract only the necessary sections.

## Examples

### Example 1

**User request:**
"Please summarize the article at https://example.com/blog/post-123"

**Agent behavior:**
- Use Sitepanda to scrape the page
- Read the extracted Markdown
- Summarize the main points in the response

### Example 2

**User request:**
"What does this documentation page say? https://example.com/docs"

**Agent behavior:**
- Fetch the page using Sitepanda
- Extract key sections
- Explain the content concisely
`
