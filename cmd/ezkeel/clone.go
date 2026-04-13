package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/internal/ai"
	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/preflight"
	"github.com/ferax564/ezkeel-cli/internal/secrets"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <project-name>",
	Short: "Clone an EZKeel project and set it up locally",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		forgejoURL, _ := cmd.Flags().GetString("forgejo-url")
		forgejoToken, _ := cmd.Flags().GetString("forgejo-token")
		env, _ := cmd.Flags().GetString("env")

		cfg := config.LoadGlobalConfig()
		forgejoURL = config.FlagOrDefault(forgejoURL, cfg.Platform.ForgejoURL)
		forgejoToken = config.FlagOrDefault(forgejoToken, cfg.Platform.ForgejoToken)

		// Build the clone URL, embedding token for private repo access
		cloneURL := projectName
		if forgejoURL != "" {
			cloneURL = buildCloneURL(forgejoURL, forgejoToken, projectName)
		}

		// Print redacted URL (never log tokens)
		displayURL := cloneURL
		if forgejoURL != "" {
			displayURL = buildCloneURL(forgejoURL, "", projectName)
		}
		fmt.Printf("Cloning %s...\n", displayURL)
		gitCmd := exec.Command("git", "clone", cloneURL)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}

		// Git clone creates a directory named after the repo (last path segment)
		localDir := filepath.Base(projectName)

		// Load the workspace from the cloned directory
		ws, err := loadWorkspaceFromDir(localDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load workspace.yaml: %v\n", err)
			// Continue without workspace — still scaffold AI config
		}

		// Inject secrets if workspace loaded successfully
		if ws != nil && ws.Secrets.InfisicalProject != "" {
			if err := preflight.RequireInfisical(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v (skipping secret injection)\n", err)
			} else {
				sc := secrets.NewClient(ws.Secrets.InfisicalProject, ws.Secrets.InfisicalURL)
				if err := sc.InjectIntoShell(env); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not inject secrets: %v\n", err)
				} else {
					fmt.Printf("Secrets injected for environment %q\n", env)
				}
			}
		}

		// Scaffold AI config
		if err := ai.ScaffoldClaudeConfig(localDir, filepath.Base(projectName)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not scaffold AI config: %v\n", err)
		}

		fmt.Printf("\nProject %q cloned and configured.\n", localDir)
		fmt.Printf("  cd %s\n", localDir)
		return nil
	},
}

// buildCloneURL constructs the git clone URL, embedding the token if provided.
// Example: https://TOKEN@git.ezkeel.com/org/repo
func buildCloneURL(baseURL, token, project string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" {
		return baseURL + "/" + project
	}
	if token != "" {
		u.User = url.User(token)
	}
	u.Path = path.Join(u.Path, project)
	return u.String()
}

func init() {
	cloneCmd.Flags().String("forgejo-url", "", "Forgejo instance base URL (e.g. https://git.ezkeel.com)")
	cloneCmd.Flags().String("forgejo-token", "", "Forgejo API token (for private repos)")
	cloneCmd.Flags().String("env", "dev", "Environment to inject secrets for")
}
