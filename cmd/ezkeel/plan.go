package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage project plans",
}

var planNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new plan document and open it in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		date := time.Now().Format("2006-01-02")
		filename := fmt.Sprintf("%s-%s.md", date, name)

		// Ensure plans/ directory exists
		if err := os.MkdirAll("plans", 0o755); err != nil {
			return fmt.Errorf("creating plans directory: %w", err)
		}

		planPath := filepath.Join("plans", filename)

		// Write template
		content := fmt.Sprintf(`# Plan: %s
Date: %s

## Problem

<!-- What problem are we solving? -->

## Proposal

<!-- How do we solve it? -->

## Success Criteria

<!-- How do we know we are done? -->

## Tasks

- [ ] Task 1

## Notes

<!-- Additional context, links, references -->
`, strings.ReplaceAll(name, "-", " "), date)

		if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing plan file: %w", err)
		}
		fmt.Printf("Created %s\n", planPath)

		// Open in $EDITOR
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		c := exec.Command(editor, planPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

var planDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show diff of plan changes since the last git tag",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the latest tag; fall back to HEAD~10 if none exists
		tagOut, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
		var since string
		if err != nil || strings.TrimSpace(string(tagOut)) == "" {
			since = "HEAD~10"
		} else {
			since = strings.TrimSpace(string(tagOut))
		}

		fmt.Printf("Plans changed since %s:\n\n", since)

		c := exec.Command("git", "diff", since, "--", "plans/")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	planCmd.AddCommand(planNewCmd)
	planCmd.AddCommand(planDiffCmd)
}
