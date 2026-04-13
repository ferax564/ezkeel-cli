package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// claudeSettings is the structure written to .claude/settings.json.
type claudeSettings struct {
	AllowedTools []string `json:"allowedTools"`
}

// claudeSettingsJSON returns a JSON string with the default Claude allowed tools list.
func claudeSettingsJSON() string {
	settings := claudeSettings{
		AllowedTools: []string{
			"Bash",
			"Edit",
			"Read",
			"Write",
			"Glob",
			"Grep",
			"WebFetch",
			"WebSearch",
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		// json.MarshalIndent on a plain struct with string fields never errors.
		panic(fmt.Sprintf("claudeSettingsJSON: unexpected marshal error: %v", err))
	}

	return string(data)
}

// ScaffoldClaudeConfig creates the .claude/ directory tree and settings.json
// inside projectDir.
func ScaffoldClaudeConfig(projectDir, projectName string) error {
	clauDir := filepath.Join(projectDir, ".claude")

	// Create required subdirectories.
	dirs := []string{
		clauDir,
		filepath.Join(clauDir, "skills"),
		filepath.Join(clauDir, "hooks"),
		filepath.Join(clauDir, "commands"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %q: %w", dir, err)
		}
	}

	// Write settings.json.
	settingsPath := filepath.Join(clauDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(claudeSettingsJSON()), 0o644); err != nil {
		return fmt.Errorf("writing settings.json for project %q: %w", projectName, err)
	}

	return nil
}
