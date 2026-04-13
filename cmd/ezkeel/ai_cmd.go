package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/ai"
	"github.com/ferax564/ezkeel-cli/internal/preflight"
	"github.com/ferax564/ezkeel-cli/internal/secrets"
	"github.com/spf13/cobra"
)

var aiCmd = &cobra.Command{
	Use:   "ai <tool> <prompt...>",
	Short: "Run an AI tool with injected secrets",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		toolArg := args[0]
		prompt := strings.Join(args[1:], " ")
		env, _ := cmd.Flags().GetString("env")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		ws, err := loadWorkspaceFromDir(cwd)
		if err != nil {
			return fmt.Errorf("loading workspace: %w", err)
		}

		// Resolve "local" shorthand to the configured local model
		tool := toolArg
		if tool == "local" {
			tool = ws.AI.Models.Local
		}

		if err := preflight.RequireAITool(tool); err != nil {
			return err
		}

		// Inject secrets if the tool requires an API key
		if ws.Secrets.InfisicalProject != "" {
			secretKey := secretKeyForTool(tool)
			if secretKey != "" && os.Getenv(secretKey) == "" {
				sc := secrets.NewClient(ws.Secrets.InfisicalProject, ws.Secrets.InfisicalURL)
				if err := sc.InjectIntoShell(env); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not inject secrets: %v\n", err)
				}
			}
		}

		// Build the launch command
		exe, launchArgs := ai.BuildLaunchCommand(tool, prompt)

		c := exec.Command(exe, launchArgs...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("running %s: %w", exe, err)
		}
		return nil
	},
}

func init() {
	aiCmd.Flags().String("env", "dev", "Environment to inject secrets for")
}
