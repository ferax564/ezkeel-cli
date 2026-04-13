package ai

import "strings"

// BuildLaunchCommand returns the executable name and argument list needed to
// launch a given AI tool with the provided prompt.
//
// Supported tool formats:
//   - "ollama/MODEL" → ("ollama", ["run", MODEL, prompt])
//   - "codex"        → ("codex",  [prompt])
//   - default        → ("claude", [prompt])
func BuildLaunchCommand(tool, prompt string) (string, []string) {
	if strings.HasPrefix(tool, "ollama/") {
		model := strings.TrimPrefix(tool, "ollama/")
		return "ollama", []string{"run", model, prompt}
	}

	if tool == "codex" {
		return "codex", []string{prompt}
	}

	return "claude", []string{prompt}
}
