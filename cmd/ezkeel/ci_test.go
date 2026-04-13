package main

import (
	"strings"
	"testing"
)

func TestFormatRunStatus(t *testing.T) {
	tests := []struct {
		status     string
		conclusion string
		want       string
	}{
		{"completed", "success", "passed"},
		{"completed", "failure", "failed"},
		{"in_progress", "", "running"},
		{"queued", "", "queued"},
		{"completed", "cancelled", "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"/"+tt.conclusion, func(t *testing.T) {
			got := formatRunStatus(tt.status, tt.conclusion)
			if got != tt.want {
				t.Errorf("formatRunStatus(%q, %q) = %q, want %q", tt.status, tt.conclusion, got, tt.want)
			}
		})
	}
}

func TestCISetupGitHub(t *testing.T) {
	content := generateGitHubMirrorWorkflow()
	if content == "" {
		t.Fatal("generateGitHubMirrorWorkflow returned empty string")
	}
	if !strings.Contains(content, "Mirror Public Paths") {
		t.Errorf("workflow missing name, got:\n%s", content)
	}
	if !strings.Contains(content, "ezkeel publish --github") {
		t.Errorf("workflow missing publish command, got:\n%s", content)
	}
	if !strings.Contains(content, "GITHUB_TOKEN") {
		t.Errorf("workflow missing GITHUB_TOKEN, got:\n%s", content)
	}
}

func TestCISetupForgejo(t *testing.T) {
	content := generateForgejoDeployWorkflow()
	if content == "" {
		t.Fatal("generateForgejoDeployWorkflow returned empty string")
	}
	if !strings.Contains(content, "ezkeel up") {
		t.Errorf("workflow missing deploy command, got:\n%s", content)
	}
	if !strings.Contains(content, "SSH_PRIVATE_KEY") {
		t.Errorf("workflow missing SSH key secret, got:\n%s", content)
	}
}
