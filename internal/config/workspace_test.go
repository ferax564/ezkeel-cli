package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/config"
)

func TestLoadWorkspace(t *testing.T) {
	yaml := `
name: my-project
version: "1.0"

visibility:
  default: private
  public_paths:
    - /public
    - /assets
  private_paths:
    - /admin
    - /internal

secrets:
  required:
    - DATABASE_URL
    - API_KEY
  infisical_project: my-infisical-project
  environments:
    - development
    - staging
    - production

ai:
  models:
    primary: claude-opus-4
    fast: claude-haiku-4
    local: ollama/llama3
  instructions: "Be concise and helpful."
  agents: AGENTS.md
  mcp_servers:
    - name: github
      command: npx
      args:
        - "@modelcontextprotocol/server-github"
      secrets:
        - GITHUB_TOKEN
    - name: stripe
      command: npx
      args:
        - "@modelcontextprotocol/server-stripe"
      secrets:
        - STRIPE_SECRET_KEY
  persona:
    skills_dir: .claude/skills
    hooks_dir: .claude/hooks
    commands_dir: .claude/commands
    settings: .claude/settings.json

environment:
  type: docker
  file: .devcontainer/devcontainer.json
  post_create:
    - npm install
    - go mod download

ci:
  provider: github-actions
  workflows_dir: .github/workflows
  on_plan_change:
    - generate-plan-diff
  on_push:
    - lint
    - test

deploy:
  provider: vercel
  environments:
    production: proj_prod123
    staging: proj_stg456
`

	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write temp workspace.yaml: %v", err)
	}

	ws, err := config.LoadWorkspace(path)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}

	// Top-level fields
	if ws.Name != "my-project" {
		t.Errorf("Name: got %q, want %q", ws.Name, "my-project")
	}
	if ws.Version != "1.0" {
		t.Errorf("Version: got %q, want %q", ws.Version, "1.0")
	}

	// Visibility
	if ws.Visibility.Default != "private" {
		t.Errorf("Visibility.Default: got %q, want %q", ws.Visibility.Default, "private")
	}
	if len(ws.Visibility.PublicPaths) != 2 || ws.Visibility.PublicPaths[0] != "/public" {
		t.Errorf("Visibility.PublicPaths: got %v", ws.Visibility.PublicPaths)
	}
	if len(ws.Visibility.PrivatePaths) != 2 || ws.Visibility.PrivatePaths[0] != "/admin" {
		t.Errorf("Visibility.PrivatePaths: got %v", ws.Visibility.PrivatePaths)
	}

	// Secrets
	if len(ws.Secrets.Required) != 2 || ws.Secrets.Required[0] != "DATABASE_URL" {
		t.Errorf("Secrets.Required: got %v", ws.Secrets.Required)
	}
	if ws.Secrets.InfisicalProject != "my-infisical-project" {
		t.Errorf("Secrets.InfisicalProject: got %q", ws.Secrets.InfisicalProject)
	}
	if len(ws.Secrets.Environments) != 3 || ws.Secrets.Environments[1] != "staging" {
		t.Errorf("Secrets.Environments: got %v", ws.Secrets.Environments)
	}

	// AI - Models
	if ws.AI.Models.Primary != "claude-opus-4" {
		t.Errorf("AI.Models.Primary: got %q", ws.AI.Models.Primary)
	}
	if ws.AI.Models.Fast != "claude-haiku-4" {
		t.Errorf("AI.Models.Fast: got %q", ws.AI.Models.Fast)
	}
	if ws.AI.Models.Local != "ollama/llama3" {
		t.Errorf("AI.Models.Local: got %q", ws.AI.Models.Local)
	}

	// AI - Instructions
	if ws.AI.Instructions != "Be concise and helpful." {
		t.Errorf("AI.Instructions: got %q", ws.AI.Instructions)
	}

	// AI - Agents
	if ws.AI.Agents != "AGENTS.md" {
		t.Errorf("AI.Agents: got %q, want %q", ws.AI.Agents, "AGENTS.md")
	}

	// AI - MCPServers
	if len(ws.AI.MCPServers) != 2 {
		t.Fatalf("AI.MCPServers: got %d, want 2", len(ws.AI.MCPServers))
	}
	if ws.AI.MCPServers[0].Name != "github" {
		t.Errorf("AI.MCPServers[0].Name: got %q", ws.AI.MCPServers[0].Name)
	}
	if ws.AI.MCPServers[0].Command != "npx" {
		t.Errorf("AI.MCPServers[0].Command: got %q", ws.AI.MCPServers[0].Command)
	}
	if len(ws.AI.MCPServers[0].Args) != 1 || ws.AI.MCPServers[0].Args[0] != "@modelcontextprotocol/server-github" {
		t.Errorf("AI.MCPServers[0].Args: got %v", ws.AI.MCPServers[0].Args)
	}
	if len(ws.AI.MCPServers[0].Secrets) != 1 || ws.AI.MCPServers[0].Secrets[0] != "GITHUB_TOKEN" {
		t.Errorf("AI.MCPServers[0].Secrets: got %v", ws.AI.MCPServers[0].Secrets)
	}

	// AI - Persona
	if ws.AI.Persona.SkillsDir != ".claude/skills" {
		t.Errorf("AI.Persona.SkillsDir: got %q", ws.AI.Persona.SkillsDir)
	}
	if ws.AI.Persona.HooksDir != ".claude/hooks" {
		t.Errorf("AI.Persona.HooksDir: got %q", ws.AI.Persona.HooksDir)
	}
	if ws.AI.Persona.CommandsDir != ".claude/commands" {
		t.Errorf("AI.Persona.CommandsDir: got %q", ws.AI.Persona.CommandsDir)
	}
	if ws.AI.Persona.Settings != ".claude/settings.json" {
		t.Errorf("AI.Persona.Settings: got %q, want %q", ws.AI.Persona.Settings, ".claude/settings.json")
	}

	// Environment
	if ws.Environment.Type != "docker" {
		t.Errorf("Environment.Type: got %q", ws.Environment.Type)
	}
	if ws.Environment.File != ".devcontainer/devcontainer.json" {
		t.Errorf("Environment.File: got %q", ws.Environment.File)
	}
	if len(ws.Environment.PostCreate) != 2 || ws.Environment.PostCreate[0] != "npm install" {
		t.Errorf("Environment.PostCreate: got %v", ws.Environment.PostCreate)
	}

	// CI
	if ws.CI.Provider != "github-actions" {
		t.Errorf("CI.Provider: got %q", ws.CI.Provider)
	}
	if ws.CI.WorkflowsDir != ".github/workflows" {
		t.Errorf("CI.WorkflowsDir: got %q", ws.CI.WorkflowsDir)
	}
	if len(ws.CI.OnPlanChange) != 1 || ws.CI.OnPlanChange[0] != "generate-plan-diff" {
		t.Errorf("CI.OnPlanChange: got %v", ws.CI.OnPlanChange)
	}
	if len(ws.CI.OnPush) != 2 || ws.CI.OnPush[0] != "lint" {
		t.Errorf("CI.OnPush: got %v", ws.CI.OnPush)
	}

	// Deploy
	if ws.Deploy.Provider != "vercel" {
		t.Errorf("Deploy.Provider: got %q", ws.Deploy.Provider)
	}
	if ws.Deploy.Environments["production"] != "proj_prod123" {
		t.Errorf("Deploy.Environments[production]: got %q", ws.Deploy.Environments["production"])
	}
	if ws.Deploy.Environments["staging"] != "proj_stg456" {
		t.Errorf("Deploy.Environments[staging]: got %q", ws.Deploy.Environments["staging"])
	}
}

func TestLoadWorkspaceMissing(t *testing.T) {
	_, err := config.LoadWorkspace("/nonexistent/path/workspace.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestFindWorkspace(t *testing.T) {
	// Create a temp directory tree: root/sub/deep
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}

	// Place workspace.yaml at root
	wsPath := filepath.Join(root, "workspace.yaml")
	if err := os.WriteFile(wsPath, []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("failed to write workspace.yaml: %v", err)
	}

	// Call FindWorkspace from the deepest directory
	found, err := config.FindWorkspace(deep)
	if err != nil {
		t.Fatalf("FindWorkspace returned error: %v", err)
	}
	if found != wsPath {
		t.Errorf("FindWorkspace: got %q, want %q", found, wsPath)
	}
}

func TestLoadWorkspace_MissingVisibility(t *testing.T) {
	yaml := "name: minimal-project\nversion: \"0.1\"\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write temp workspace.yaml: %v", err)
	}

	ws, err := config.LoadWorkspace(path)
	if err != nil {
		t.Fatalf("LoadWorkspace returned error: %v", err)
	}
	if ws.Visibility.Default != "" {
		t.Errorf("Visibility.Default: got %q, want empty string", ws.Visibility.Default)
	}
	if len(ws.Visibility.PublicPaths) != 0 {
		t.Errorf("Visibility.PublicPaths: got %v, want empty", ws.Visibility.PublicPaths)
	}
	if len(ws.Visibility.PrivatePaths) != 0 {
		t.Errorf("Visibility.PrivatePaths: got %v, want empty", ws.Visibility.PrivatePaths)
	}
}

func TestLoadWorkspace_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatalf("failed to write temp workspace.yaml: %v", err)
	}

	_, err := config.LoadWorkspace(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestFindWorkspace_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := config.FindWorkspace(dir)
	if err == nil {
		t.Fatal("expected error when no workspace.yaml exists, got nil")
	}
}
