package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// SkillInstallHandler is a function that handles the skill installation logic.
// It will be set by the main package.
var SkillInstallHandler func() error

// skillCmd represents the skill command
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage AI coding tool skills",
	Long:  `Manage skills for AI coding tools like OpenAI Codex and Claude Code.`,
}

// skillInstallCmd represents the skill install command
var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Sitepanda skill",
	Long:  `Install the Sitepanda skill for OpenAI Codex or Claude Code.`,
	Run: func(cmd *cobra.Command, args []string) {
		if SkillInstallHandler != nil {
			if err := SkillInstallHandler(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Error: Skill install handler not set.")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillInstallCmd)
}
