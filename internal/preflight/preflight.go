package preflight

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckCommand verifies that a CLI tool is available on PATH.
// Returns a user-friendly error with installHint if the tool is missing.
func CheckCommand(name, installHint string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%q not found on PATH. %s", name, installHint)
	}
	return nil
}

// RequireInfisical checks that the Infisical CLI is installed.
func RequireInfisical() error {
	return CheckCommand("infisical",
		"Install the Infisical CLI: https://infisical.com/docs/cli/overview")
}

// RequireAITool checks that the CLI for the given AI tool is installed.
// Returns nil for tools that don't require a CLI (e.g. ollama models are optional).
func RequireAITool(tool string) error {
	switch {
	case tool == "claude":
		return CheckCommand("claude",
			"Install Claude Code: npm install -g @anthropic-ai/claude-code")
	case tool == "codex":
		return CheckCommand("codex",
			"Install Codex: npm install -g @openai/codex")
	case strings.HasPrefix(tool, "ollama"):
		return nil
	default:
		return nil
	}
}
