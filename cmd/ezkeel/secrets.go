package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/preflight"
	"github.com/ferax564/ezkeel-cli/internal/secrets"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage project secrets via Infisical",
}

var secretsInjectCmd = &cobra.Command{
	Use:   "inject <environment>",
	Short: "Export secrets for the given environment to stdout as shell export statements",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]

		if err := preflight.RequireInfisical(); err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		ws, err := loadWorkspaceFromDir(cwd)
		if err != nil {
			return fmt.Errorf("loading workspace: %w", err)
		}

		if ws.Secrets.InfisicalProject == "" {
			return fmt.Errorf("secrets.infisical_project is not set in workspace.yaml")
		}

		sc := secrets.NewClient(ws.Secrets.InfisicalProject, ws.Secrets.InfisicalURL)
		kvs, err := sc.Export(env)
		if err != nil {
			return fmt.Errorf("exporting secrets: %w", err)
		}

		// Print export statements to stdout
		for k, v := range kvs {
			fmt.Printf("export %s=%q\n", k, v)
		}

		// Print eval hint to stderr
		fmt.Fprintf(os.Stderr, "\n# Hint: run `eval $(ezkeel secrets inject %s)` to load these into your shell\n", env)
		return nil
	},
}

func init() {
	secretsCmd.AddCommand(secretsInjectCmd)
}

// secretKeyForTool maps an AI tool name to the environment variable that holds its API key.
// Returns an empty string if the tool does not require a dedicated API key secret.
func secretKeyForTool(tool string) string {
	switch {
	case tool == "claude":
		return "ANTHROPIC_API_KEY"
	case tool == "codex":
		return "OPENAI_API_KEY"
	case strings.HasPrefix(tool, "ollama"):
		return ""
	default:
		return ""
	}
}
