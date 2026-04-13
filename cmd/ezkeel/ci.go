package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/pkg/forgejo"
	"github.com/spf13/cobra"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Manage CI/CD workflows",
}

var ciStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show recent CI workflow runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		forgejoURL, _ := cmd.Flags().GetString("forgejo-url")
		forgejoToken, _ := cmd.Flags().GetString("forgejo-token")
		owner, _ := cmd.Flags().GetString("owner")
		repo, _ := cmd.Flags().GetString("repo")

		cfg := config.LoadGlobalConfig()
		forgejoURL = config.FlagOrDefault(forgejoURL, cfg.Platform.ForgejoURL)
		forgejoToken = config.FlagOrDefault(forgejoToken, cfg.Platform.ForgejoToken)
		owner = config.FlagOrDefault(owner, cfg.Platform.Owner)

		if forgejoURL == "" || forgejoToken == "" || owner == "" || repo == "" {
			return fmt.Errorf("--forgejo-url, --forgejo-token, --owner, and --repo are required")
		}

		client := forgejo.NewClient(forgejoURL, forgejoToken)
		runs, err := client.ListWorkflowRuns(owner, repo, 5)
		if err != nil {
			return err
		}

		if len(runs) == 0 {
			fmt.Println("No workflow runs found.")
			return nil
		}

		fmt.Printf("%-8s %-12s %-10s %-20s %s\n", "ID", "STATUS", "BRANCH", "CREATED", "URL")
		for _, r := range runs {
			status := formatRunStatus(r.Status, r.Conclusion)
			created := r.CreatedAt
			if len(created) > 19 {
				created = created[:19]
			}
			fmt.Printf("%-8d %-12s %-10s %-20s %s\n", r.ID, status, r.HeadBranch, created, r.HTMLURL)
		}

		return nil
	},
}

// formatRunStatus returns a human-readable status string.
func formatRunStatus(status, conclusion string) string {
	switch status {
	case "completed":
		switch conclusion {
		case "success":
			return "passed"
		case "failure":
			return "failed"
		default:
			return conclusion
		}
	case "in_progress":
		return "running"
	default:
		return status
	}
}

// generateForgejoDeployWorkflow returns the Forgejo Actions deploy workflow template.
func generateForgejoDeployWorkflow() string {
	return `name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install ezkeel
        run: |
          curl -fsSL https://ezkeel.com/install.sh | sh
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Configure SSH
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.SSH_PRIVATE_KEY }}" > ~/.ssh/deploy_key
          chmod 600 ~/.ssh/deploy_key
          ssh-keyscan -H ${{ vars.DEPLOY_HOST }} >> ~/.ssh/known_hosts 2>/dev/null

      - name: Configure ezkeel
        run: |
          ezkeel server add \
            --host "${{ vars.DEPLOY_HOST }}" \
            --domain "${{ vars.DEPLOY_DOMAIN }}" \
            --key ~/.ssh/deploy_key \
            --name deploy-target

      - name: Deploy
        run: ezkeel up --server deploy-target --plain
`
}

// generateGitHubMirrorWorkflow returns the GitHub Actions mirror workflow template.
func generateGitHubMirrorWorkflow() string {
	return `name: Mirror Public Paths

on:
  push:
    branches: [main]

jobs:
  mirror:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install ezkeel
        run: go install github.com/ferax564/ezkeel-cli/cmd/ezkeel@latest

      - name: Publish public mirror
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: ezkeel publish --github
        # Note: The default GITHUB_TOKEN can create repos under the same owner.
        # For cross-org repos, create a PAT with repo scope and add it as
        # a repository secret named GITHUB_TOKEN (overriding the default).
`
}

var ciSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate a CI workflow for auto-deploy or mirror",
	RunE: func(cmd *cobra.Command, args []string) error {
		githubFlag, _ := cmd.Flags().GetBool("github")

		var outPath, content string
		if githubFlag {
			outPath = ".github/workflows/mirror.yaml"
			content = generateGitHubMirrorWorkflow()
		} else {
			outPath = ".forgejo/workflows/deploy.yaml"
			content = generateForgejoDeployWorkflow()
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("creating workflow directory: %w", err)
		}
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing workflow: %w", err)
		}

		fmt.Printf("Created %s\n\n", outPath)
		fmt.Println("Next steps:")
		if githubFlag {
			fmt.Println("  1. Ensure GITHUB_TOKEN has write access or add a PAT as GITHUB_TOKEN secret")
			fmt.Println("  2. Commit and push to trigger the first mirror run")
		} else {
			fmt.Println("  1. Add your SSH private key as a Forgejo secret named SSH_PRIVATE_KEY")
			fmt.Println("  2. Add DEPLOY_HOST and DEPLOY_DOMAIN as repository variables")
			fmt.Println("  3. Commit and push to trigger the first deploy")
		}
		return nil
	},
}

func init() {
	ciStatusCmd.Flags().String("forgejo-url", "", "Forgejo instance URL")
	ciStatusCmd.Flags().String("forgejo-token", "", "Forgejo API token")
	ciStatusCmd.Flags().String("owner", "", "Repository owner")
	ciStatusCmd.Flags().String("repo", "", "Repository name")

	ciSetupCmd.Flags().Bool("github", false, "Generate GitHub Actions mirror workflow instead of Forgejo deploy workflow")

	ciCmd.AddCommand(ciStatusCmd)
	ciCmd.AddCommand(ciSetupCmd)
}
