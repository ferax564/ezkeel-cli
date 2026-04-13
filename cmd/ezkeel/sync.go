package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the AI persona configuration with the remote",
}

var syncPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push the local .claude/ persona to the ezkeel/persona branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Stash any in-progress changes (ignore error — nothing to stash is fine)
		runGit("stash")

		// Check out (or create) the ezkeel/persona branch
		if err := runGit("checkout", "-B", "ezkeel/persona"); err != nil {
			return fmt.Errorf("git checkout -B ezkeel/persona: %w", err)
		}

		// Stage the .claude/ directory
		if err := runGit("add", ".claude/"); err != nil {
			return fmt.Errorf("git add .claude/: %w", err)
		}

		// Commit — ignore error if there is nothing to commit
		runGit("commit", "-m", "chore: sync persona configuration")

		// Force-push
		if err := runGit("push", "--force", "origin", "ezkeel/persona"); err != nil {
			return fmt.Errorf("git push: %w", err)
		}

		// Return to previous branch
		runGit("checkout", "-")

		// Restore stashed changes (ignore error — nothing stashed is fine)
		runGit("stash", "pop")

		fmt.Println("Persona pushed to origin/ezkeel/persona")
		return nil
	},
}

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the AI persona from the ezkeel/persona branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fetch the remote branch
		if err := runGit("fetch", "origin", "ezkeel/persona"); err != nil {
			return fmt.Errorf("git fetch origin ezkeel/persona: %w", err)
		}

		// Checkout just the .claude/ subtree from the remote branch
		if err := runGit("checkout", "origin/ezkeel/persona", "--", ".claude/"); err != nil {
			return fmt.Errorf("git checkout origin/ezkeel/persona -- .claude/: %w", err)
		}

		fmt.Println("Persona pulled from origin/ezkeel/persona")
		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)
}

// runGit runs a git command with stdout/stderr passed through.
// It returns the error but callers may choose to ignore it for optional steps.
func runGit(args ...string) error {
	c := exec.Command("git", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
