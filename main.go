package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skills/git-wtkit/SKILL.md
var skillText string

func installSkill(cmd *cobra.Command, _ []string) error {
	config, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(config, "opencode", "skills", "git-wtkit")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "SKILL.md")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if errors.Is(err, os.ErrExist) {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(existing) != skillText {
			return fmt.Errorf("%s has different contents; leaving it unchanged", path)
		}
	} else if err != nil {
		return err
	} else {
		_, err := f.WriteString(skillText)
		if err := errors.Join(err, f.Close()); err != nil {
			os.Remove(path)
			return err
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Skill installed:", path)
	return nil
}

func main() {
	root := &cobra.Command{
		Use:          "git-wtkit",
		Short:        "Initialize submodules using matching objects from sibling worktrees",
		Example:      "  git wtkit install skill\n  git wtkit install\n  git wtkit setup",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	install := &cobra.Command{
		Use:   "install",
		Short: "Register 'git wtsetup' in your global Git config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			const key, value = "alias.wtsetup", "wtkit setup"
			old, err := git(".", "config", "--global", "--includes", "--get-all", key)
			var exit *exec.ExitError
			switch {
			case err == nil && old == "wtinit setup":
				if _, err := git(".", "config", "--global", "--replace-all", key, value, "^wtinit setup$"); err != nil {
					return err
				}
			case err == nil && old != value:
				return fmt.Errorf("%s already exists; leaving it unchanged", key)
			case err == nil:
			case errors.As(err, &exit) && exit.ExitCode() == 1:
				if _, err := git(".", "config", "--global", key, value); err != nil {
					return err
				}
			default:
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Configured: git wtsetup = git wtkit setup")
			return nil
		},
	}
	install.AddCommand(&cobra.Command{
		Use:   "hook",
		Short: "Prepare submodules after worktree creation and branch checkout in this repository",
		Args:  cobra.NoArgs,
		RunE:  installHook,
	}, &cobra.Command{
		Use:   "skill",
		Short: "Install the bundled OpenCode skill for this user",
		Args:  cobra.NoArgs,
		RunE:  installSkill,
	})
	root.AddCommand(install, &cobra.Command{
		Use:   "setup",
		Short: "Initialize submodules in the current worktree",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return run() },
	})
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
