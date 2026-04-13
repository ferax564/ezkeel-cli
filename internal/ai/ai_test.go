package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeSettingsJSON(t *testing.T) {
	result := claudeSettingsJSON()
	if result == "" {
		t.Fatal("claudeSettingsJSON() returned empty string")
	}
	if !strings.Contains(result, "allowedTools") {
		t.Errorf("claudeSettingsJSON() output does not contain 'allowedTools'; got: %s", result)
	}
}

func TestScaffoldClaudeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	err := ScaffoldClaudeConfig(tmpDir, "test-project")
	if err != nil {
		t.Fatalf("ScaffoldClaudeConfig() returned error: %v", err)
	}

	settingsFile := filepath.Join(tmpDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		t.Errorf(".claude/settings.json does not exist at %s", settingsFile)
	}

	dirsToCheck := []string{
		filepath.Join(tmpDir, ".claude", "skills"),
		filepath.Join(tmpDir, ".claude", "hooks"),
		filepath.Join(tmpDir, ".claude", "commands"),
	}
	for _, dir := range dirsToCheck {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			t.Errorf("expected directory %s does not exist", dir)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestBuildLaunchCommand(t *testing.T) {
	tests := []struct {
		tool     string
		prompt   string
		wantCmd  string
		wantArgs []string
	}{
		{
			tool:     "claude",
			prompt:   "fix bug",
			wantCmd:  "claude",
			wantArgs: []string{"fix bug"},
		},
		{
			tool:     "codex",
			prompt:   "write tests",
			wantCmd:  "codex",
			wantArgs: []string{"write tests"},
		},
		{
			tool:     "ollama/qwen3-coder",
			prompt:   "explain",
			wantCmd:  "ollama",
			wantArgs: []string{"run", "qwen3-coder", "explain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			gotCmd, gotArgs := BuildLaunchCommand(tt.tool, tt.prompt)
			if gotCmd != tt.wantCmd {
				t.Errorf("BuildLaunchCommand(%q, %q) cmd = %q, want %q", tt.tool, tt.prompt, gotCmd, tt.wantCmd)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("BuildLaunchCommand(%q, %q) args len = %d, want %d; got %v", tt.tool, tt.prompt, len(gotArgs), len(tt.wantArgs), gotArgs)
				return
			}
			for i, arg := range gotArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("BuildLaunchCommand(%q, %q) args[%d] = %q, want %q", tt.tool, tt.prompt, i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}
