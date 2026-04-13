package tui

import (
	"strings"
	"testing"
)

func TestDeployModel_StepRendering(t *testing.T) {
	m := NewDeployModel("my-app", []string{"Clone", "Detect", "Build", "Deploy"})
	view := m.View()
	if !strings.Contains(view, "Clone") {
		t.Error("expected step text")
	}
	if !strings.Contains(view, "my-app") {
		t.Error("expected app name")
	}
}

func TestDeployModel_CompleteStep(t *testing.T) {
	m := NewDeployModel("my-app", []string{"Clone", "Detect"})
	m.CompleteStep(0, "Cloned repository")
	m.StartStep(1)
	view := m.View()
	if !strings.Contains(view, IconDone) {
		t.Error("expected done icon")
	}
	if !strings.Contains(view, IconActive) {
		t.Error("expected active icon")
	}
}

func TestDeployModel_FailStep(t *testing.T) {
	m := NewDeployModel("my-app", []string{"Clone", "Build"})
	m.CompleteStep(0, "Cloned")
	m.FailStep(1, "npm run build failed")
	view := m.View()
	if !strings.Contains(view, IconFail) {
		t.Error("expected fail icon")
	}
	if !strings.Contains(view, "npm run build failed") {
		t.Error("expected error msg")
	}
}

func TestDeployModel_SuccessView(t *testing.T) {
	result := &DeployResult{
		AppName: "my-app",
		URL:     "https://my-app.deploy.example.com",
		Server:  "vps01",
		Stack:   "Next.js",
		TimeSec: 47,
	}
	view := RenderSuccess(result)
	if !strings.Contains(view, "my-app") {
		t.Error("expected app name")
	}
	if !strings.Contains(view, "https://my-app.deploy.example.com") {
		t.Error("expected URL")
	}
	if !strings.Contains(view, "47s") {
		t.Error("expected time")
	}
}
