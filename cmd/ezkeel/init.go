package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/ai"
	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/pkg/forgejo"
	infisicalAdmin "github.com/ferax564/ezkeel-cli/internal/infisical"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <project-name>",
	Short: "Initialize a new EZKeel project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		forgejoURL, _ := cmd.Flags().GetString("forgejo-url")
		forgejoToken, _ := cmd.Flags().GetString("forgejo-token")

		cfg := config.LoadGlobalConfig()
		forgejoURL = config.FlagOrDefault(forgejoURL, cfg.Platform.ForgejoURL)
		forgejoToken = config.FlagOrDefault(forgejoToken, cfg.Platform.ForgejoToken)

		projectDir, err := filepath.Abs(projectName)
		if err != nil {
			return fmt.Errorf("resolving project dir: %w", err)
		}

		// Create repos on Forgejo if credentials are provided
		if forgejoURL != "" && forgejoToken != "" {
			client := forgejo.NewClient(forgejoURL, forgejoToken)
			repo, err := client.CreateRepo(projectName, "EZKeel project: "+projectName, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not create Forgejo repo: %v\n", err)
			} else {
				fmt.Printf("Created repository: %s\n", repo.CloneURL)
				// Set up push webhook if a webhook URL is provided
				webhookURL, _ := cmd.Flags().GetString("webhook-url")
				if webhookURL != "" {
					// Extract owner from clone URL: https://host/OWNER/repo.git
					repoOwner := "ezkeel-admin"
					if parts := strings.Split(strings.TrimSuffix(repo.CloneURL, ".git"), "/"); len(parts) >= 2 {
						repoOwner = parts[len(parts)-2]
					}
					if err := client.CreateWebhook(repoOwner, projectName, webhookURL, []string{"push"}); err != nil {
						fmt.Fprintf(os.Stderr, "warning: could not create webhook: %v\n", err)
					} else {
						fmt.Printf("Created push webhook: %s\n", webhookURL)
					}
				}
			}
		}

		// Create Infisical project if credentials are provided
		infisicalURL, _ := cmd.Flags().GetString("infisical-url")
		infisicalOrg, _ := cmd.Flags().GetString("infisical-org")
		infisicalClientID, _ := cmd.Flags().GetString("infisical-client-id")
		infisicalClientSecret, _ := cmd.Flags().GetString("infisical-client-secret")

		infisicalURL = config.FlagOrDefault(infisicalURL, cfg.Platform.InfisicalURL)
		infisicalOrg = config.FlagOrDefault(infisicalOrg, cfg.Platform.InfisicalOrg)
		infisicalClientID = config.FlagOrDefault(infisicalClientID, cfg.Platform.InfisicalClientID)
		infisicalClientSecret = config.FlagOrDefault(infisicalClientSecret, cfg.Platform.InfisicalClientSecret)

		if infisicalURL != "" && infisicalOrg != "" && infisicalClientID != "" && infisicalClientSecret != "" {
			ic, err := infisicalAdmin.LoginUniversalAuth(infisicalURL, infisicalClientID, infisicalClientSecret)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not authenticate with Infisical: %v\n", err)
			} else {
				project, err := ic.CreateProject(projectName, infisicalOrg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not create Infisical project: %v\n", err)
				} else {
					fmt.Printf("Created Infisical project: %s (ID: %s)\n", project.Name, project.ID)
				}
			}
		}

		// Scaffold the project directory
		if err := scaffoldProject(projectDir, projectName); err != nil {
			return fmt.Errorf("scaffolding project: %w", err)
		}

		// Scaffold Claude AI config
		if err := ai.ScaffoldClaudeConfig(projectDir, projectName); err != nil {
			return fmt.Errorf("scaffolding Claude config: %w", err)
		}

		fmt.Printf("\nProject %q initialized at %s\n", projectName, projectDir)
		fmt.Println("Next steps:")
		fmt.Println("  cd", projectName)
		fmt.Println("  ezkeel secrets inject dev")
		return nil
	},
}

func init() {
	initCmd.Flags().String("forgejo-url", "", "Forgejo instance URL (e.g. https://git.ezkeel.com)")
	initCmd.Flags().String("forgejo-token", "", "Forgejo API token")
	initCmd.Flags().String("infisical-url", "", "Infisical instance URL (e.g. https://secrets.ezkeel.com)")
	initCmd.Flags().String("infisical-client-id", "", "Infisical machine identity client ID")
	initCmd.Flags().String("infisical-client-secret", "", "Infisical machine identity client secret")
	initCmd.Flags().String("infisical-org", "", "Infisical organization ID")
	initCmd.Flags().String("webhook-url", "", "URL to receive push webhooks for auto-deploy")
}

// scaffoldProject creates the standard EZKeel project directory structure.
func scaffoldProject(projectDir, projectName string) error {
	// Create directory structure
	dirs := []string{
		projectDir,
		filepath.Join(projectDir, ".devcontainer"),
		filepath.Join(projectDir, ".forgejo", "workflows"),
		filepath.Join(projectDir, "plans"),
		filepath.Join(projectDir, "docs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating directory %q: %w", d, err)
		}
	}

	// Files to scaffold: map of relative path → template name
	files := map[string]string{
		"workspace.yaml":                  "workspace.yaml",
		"ezkeel.yaml":                     "ezkeel.yaml",
		"CLAUDE.md":                       "CLAUDE.md",
		"AGENTS.md":                       "AGENTS.md",
		".devcontainer/devcontainer.json": "devcontainer.json",
		".forgejo/workflows/ci.yaml":      "ci.yaml",
		".gitignore":                      "gitignore",
	}

	for relPath, tmplName := range files {
		content := readTemplate(tmplName)
		content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", projectName)

		dst := filepath.Join(projectDir, relPath)
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %q: %w", dst, err)
		}
		fmt.Printf("  created %s\n", relPath)
	}

	return nil
}

// readTemplate reads a template file by name, trying several locations.
func readTemplate(name string) string {
	candidates := []string{
		filepath.Join("templates", name),
		filepath.Join("/usr/local/share/ezkeel/templates", name),
		filepath.Join("/opt/ezkeel/templates", name),
	}

	// Try executable-relative path
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append([]string{filepath.Join(exeDir, "templates", name)}, candidates...)
	}

	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err == nil {
			return string(data)
		}
	}

	// Return a minimal fallback so the command still works
	return defaultTemplate(name)
}

// defaultTemplate returns a minimal inline template when the file isn't found.
func defaultTemplate(name string) string {
	switch name {
	case "ezkeel.yaml":
		return `# spec: ezkeel/v1
name: {{PROJECT_NAME}}
`
	case "workspace.yaml":
		return `name: {{PROJECT_NAME}}
version: "1.0"

visibility:
  default: private
  public_paths:
    - src/
    - docs/
    - tests/
    - README.md
    - LICENSE
    - CONTRIBUTING.md
  private_paths:
    - plans/
    - internal/
    - .claude/
    - workspace.yaml

secrets:
  required:
    - ANTHROPIC_API_KEY
  infisical_project: {{PROJECT_NAME}}
  infisical_url: ""
  environments:
    - dev
    - staging
    - prod

ai:
  models:
    primary: claude-opus-4-6
    fast: claude-sonnet-4-6
    local: ollama/qwen3-coder
  instructions: CLAUDE.md
  agents: AGENTS.md
  mcp_servers:
    - name: filesystem
      command: npx -y @modelcontextprotocol/server-filesystem
      args: ["."]
  persona:
    skills_dir: .claude/skills/
    hooks_dir: .claude/hooks/
    commands_dir: .claude/commands/
    settings: .claude/settings.json

environment:
  type: devcontainer
  file: .devcontainer/devcontainer.json
  post_create:
    - ezkeel secrets inject dev

ci:
  provider: forgejo-actions
  workflows_dir: .forgejo/workflows/
  on_push:
    - lint
    - test

pages:
  domain: ""
  build_dir: docs/
  build_cmd: ""

deploy:
  provider: docker
  environments:
    prod: ""
`
	case "CLAUDE.md":
		return `# {{PROJECT_NAME}}

This project is managed by EZKeel.

## Workflow

- Use ` + "`ezkeel secrets inject <env>`" + ` to load environment variables
- Use ` + "`ezkeel ai <tool> <prompt>`" + ` to run AI tools with injected secrets
- Use ` + "`ezkeel sync push`" + ` to sync your persona configuration
`
	case "AGENTS.md":
		return `# Agents — {{PROJECT_NAME}}

This file describes the AI agents configured for this project.

## Available Agents

- **claude** — Primary AI assistant (Claude Opus)
- **fast** — Fast assistant for quick tasks (Claude Haiku)
- **local** — Local model via Ollama
`
	case "devcontainer.json":
		return `{
  "name": "{{PROJECT_NAME}}",
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": {
    "ghcr.io/devcontainers/features/node:1": {},
    "ghcr.io/devcontainers/features/go:1": {}
  },
  "postCreateCommand": "ezkeel secrets inject dev"
}
`
	case "ci.yaml":
		return `name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: docker
    container:
      image: golang:1.26
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: go build ./...
      - name: Test
        run: go test ./...
      - name: Vet
        run: go vet ./...
`
	case "gitignore":
		return `.env
.env.local
*.secret
node_modules/
dist/
build/
`
	default:
		return fmt.Sprintf("# %s\n# Generated by EZKeel for project {{PROJECT_NAME}}\n", name)
	}
}

// loadWorkspaceFromDir finds and loads workspace.yaml from the given directory.
func loadWorkspaceFromDir(dir string) (*config.Workspace, error) {
	wsPath, err := config.FindWorkspace(dir)
	if err != nil {
		return nil, err
	}
	return config.LoadWorkspace(wsPath)
}
