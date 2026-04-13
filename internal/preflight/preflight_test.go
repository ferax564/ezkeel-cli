package preflight

import (
	"strings"
	"testing"
)

func TestCheckCommandFound(t *testing.T) {
	// "go" should always be available in a Go test environment
	err := CheckCommand("go", "go is required: install from https://go.dev")
	if err != nil {
		t.Errorf("CheckCommand('go') returned error: %v", err)
	}
}

func TestCheckCommandNotFound(t *testing.T) {
	err := CheckCommand("ezkeel-nonexistent-tool-abc123", "install it from https://example.com")
	if err == nil {
		t.Fatal("CheckCommand for nonexistent tool should return error")
	}
	if !strings.Contains(err.Error(), "ezkeel-nonexistent-tool-abc123") {
		t.Errorf("error should mention tool name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://example.com") {
		t.Errorf("error should include install hint, got: %v", err)
	}
}

func TestRequireInfisical(t *testing.T) {
	err := RequireInfisical()
	if err != nil {
		if !strings.Contains(err.Error(), "infisical") {
			t.Errorf("RequireInfisical error should mention 'infisical', got: %v", err)
		}
	}
}

func TestRequireAIToolOllama(t *testing.T) {
	// Ollama tools should never require a CLI check
	err := RequireAITool("ollama/qwen3-coder")
	if err != nil {
		t.Errorf("RequireAITool for ollama should not error: %v", err)
	}
}
