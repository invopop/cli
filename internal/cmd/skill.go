package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/invocli/internal/skills"
	"github.com/spf13/cobra"
)

func skillCmd() *cobra.Command {
	skill := &cobra.Command{
		Use:   "skill",
		Short: "Manage the invocli AI skill for Claude Code",
	}

	skill.AddCommand(skillInstallCmd())
	skill.AddCommand(skillShowCmd())

	return skill
}

func skillInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the invocli skill to ~/.claude/skills/invopop/",
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("finding home directory: %w", err)
			}

			dir := filepath.Join(home, ".claude", "skills", "invopop")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating skill directory: %w", err)
			}

			path := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(path, skills.SkillContent, 0o644); err != nil {
				return fmt.Errorf("writing skill file: %w", err)
			}

			_, _ = fmt.Fprintf(os.Stdout, "Skill installed to %s\n", path)
			_, _ = fmt.Fprintln(os.Stdout, "You can now use /invocli in Claude Code to manage your Invopop account.")
			return nil
		},
	}
}

func skillShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the skill content to stdout",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := os.Stdout.Write(skills.SkillContent)
			return err
		},
	}
}
