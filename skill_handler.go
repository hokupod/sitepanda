package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HandleSkillInstall handles the interactive installation of the Sitepanda skill
func HandleSkillInstall() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Which AI coding tool do you want to install the Sitepanda skill for?")
	fmt.Println()
	fmt.Println("1) OpenAI Codex")
	fmt.Println("2) Claude Code")
	fmt.Println()
	fmt.Print("Enter your choice (1 or 2): ")

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
		if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
			installPath = filepath.Join(codexHome, "skills", "sitepanda")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			installPath = filepath.Join(homeDir, ".codex", "skills", "sitepanda")
		}
	case "2":
		toolName = "Claude Code"
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		installPath = filepath.Join(homeDir, ".claude", "skills", "sitepanda")
	default:
		return fmt.Errorf("invalid choice: %s", choiceStr)
	}

	// Check if directory already exists
	if _, err := os.Stat(installPath); err == nil {
		fmt.Printf("Warning: The directory %s already exists.\n", installPath)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		overwriteStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		overwriteStr = strings.TrimSpace(strings.ToLower(overwriteStr))
		if overwriteStr != "y" && overwriteStr != "yes" {
			fmt.Println("Installation cancelled.")
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

	fmt.Printf("✔ Sitepanda skill installed for %s\n", toolName)
	fmt.Printf("  Path: %s\n\n", installPath)
	fmt.Printf("Restart %s to load the new skill.\n", toolName)

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
2. Execute the following command:

   sitepanda scrape <URL> --silent

3. Capture the output, which is returned in Markdown format.
4. Read and analyze the extracted content.
5. Respond to the user using only the relevant information from the page.
6. If the content is long, summarize or extract only the necessary sections.

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
