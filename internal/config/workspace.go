package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Workspace is the top-level configuration loaded from workspace.yaml.
type Workspace struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Visibility  VisibilityConfig  `yaml:"visibility"`
	Secrets     SecretsConfig     `yaml:"secrets"`
	AI          AIConfig          `yaml:"ai"`
	Environment EnvironmentConfig `yaml:"environment"`
	CI          CIConfig          `yaml:"ci"`
	Deploy      DeployConfig      `yaml:"deploy"`
	Pages       PagesConfig       `yaml:"pages"`
}

// VisibilityConfig controls which paths are public or private.
type VisibilityConfig struct {
	Default      string   `yaml:"default"`
	PublicPaths  []string `yaml:"public_paths"`
	PrivatePaths []string `yaml:"private_paths"`
}

// SecretsConfig defines required secrets and their sources.
type SecretsConfig struct {
	Required         []string `yaml:"required"`
	InfisicalProject string   `yaml:"infisical_project"`
	InfisicalURL     string   `yaml:"infisical_url"`
	Environments     []string `yaml:"environments"`
}

// AIConfig holds AI model and tooling configuration.
type AIConfig struct {
	Models       ModelsConfig  `yaml:"models"`
	Instructions string        `yaml:"instructions"`
	Agents       string        `yaml:"agents"`
	MCPServers   []MCPServer   `yaml:"mcp_servers"`
	Persona      PersonaConfig `yaml:"persona"`
}

// ModelsConfig specifies which LLM models to use for different tasks.
type ModelsConfig struct {
	Primary string `yaml:"primary"`
	Fast    string `yaml:"fast"`
	Local   string `yaml:"local"`
}

// MCPServer represents an MCP (Model Context Protocol) server configuration.
type MCPServer struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Secrets []string `yaml:"secrets"`
}

// PersonaConfig defines the AI persona directory layout and settings.
type PersonaConfig struct {
	SkillsDir   string `yaml:"skills_dir"`
	HooksDir    string `yaml:"hooks_dir"`
	CommandsDir string `yaml:"commands_dir"`
	Settings    string `yaml:"settings"`
}

// EnvironmentConfig describes the development environment setup.
type EnvironmentConfig struct {
	Type       string   `yaml:"type"`
	File       string   `yaml:"file"`
	PostCreate []string `yaml:"post_create"`
}

// CIConfig holds continuous integration settings.
type CIConfig struct {
	Provider     string   `yaml:"provider"`
	WorkflowsDir string   `yaml:"workflows_dir"`
	OnPlanChange []string `yaml:"on_plan_change"`
	OnPush       []string `yaml:"on_push"`
}

// DeployConfig defines deployment provider and target environments.
type DeployConfig struct {
	Provider     string            `yaml:"provider"`
	Environments map[string]string `yaml:"environments"`
}

// PagesConfig defines static site hosting settings.
type PagesConfig struct {
	Domain   string `yaml:"domain"`    // e.g. "myproject.ezkeel.com"
	BuildDir string `yaml:"build_dir"` // e.g. "dist/" or "docs/"
	BuildCmd string `yaml:"build_cmd"` // e.g. "npm run build" (optional)
}

// LoadWorkspace reads and parses a workspace.yaml file at the given path.
func LoadWorkspace(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workspace file %q: %w", path, err)
	}

	var ws Workspace
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parsing workspace file %q: %w", path, err)
	}

	return &ws, nil
}

// FindWorkspace walks up from startDir looking for a workspace.yaml file.
// It returns the absolute path to the first workspace.yaml found, or an error
// if none is found before reaching the filesystem root.
func FindWorkspace(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "workspace.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding workspace.yaml
			return "", fmt.Errorf("workspace.yaml not found in %q or any parent directory", startDir)
		}
		dir = parent
	}
}
