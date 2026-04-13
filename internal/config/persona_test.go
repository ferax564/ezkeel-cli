package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyPersona(t *testing.T) {
	// Create source tmpdir with .claude/settings.json and .claude/skills/test.md
	srcBase := t.TempDir()
	srcClaudeDir := filepath.Join(srcBase, ".claude")

	if err := os.MkdirAll(filepath.Join(srcClaudeDir, "skills"), 0755); err != nil {
		t.Fatalf("creating src dirs: %v", err)
	}

	settingsContent := `{"key":"val"}`
	if err := os.WriteFile(filepath.Join(srcClaudeDir, "settings.json"), []byte(settingsContent), 0644); err != nil {
		t.Fatalf("writing settings.json: %v", err)
	}

	skillContent := "# Skill"
	if err := os.WriteFile(filepath.Join(srcClaudeDir, "skills", "test.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("writing test.md: %v", err)
	}

	// Create destination tmpdir
	dstBase := t.TempDir()
	dstClaudeDir := filepath.Join(dstBase, ".claude")

	// Call CopyPersona
	if err := CopyPersona(srcClaudeDir, dstClaudeDir); err != nil {
		t.Fatalf("CopyPersona: %v", err)
	}

	// Assert dst/.claude/settings.json has correct content
	gotSettings, err := os.ReadFile(filepath.Join(dstClaudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("reading dst settings.json: %v", err)
	}
	if string(gotSettings) != settingsContent {
		t.Errorf("settings.json content = %q, want %q", string(gotSettings), settingsContent)
	}

	// Assert dst/.claude/skills/test.md has correct content
	gotSkill, err := os.ReadFile(filepath.Join(dstClaudeDir, "skills", "test.md"))
	if err != nil {
		t.Fatalf("reading dst skills/test.md: %v", err)
	}
	if string(gotSkill) != skillContent {
		t.Errorf("skills/test.md content = %q, want %q", string(gotSkill), skillContent)
	}
}
