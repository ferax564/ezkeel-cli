package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/spec"
	"github.com/ferax564/ezkeel-cli/pkg/templates"
)

func TestAppNameFromRepo(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/user/my-app", "my-app"},
		{"github.com/user/my-app.git", "my-app"},
		{"https://github.com/user/my-app", "my-app"},
		{"https://github.com/user/my-app.git", "my-app"},
		{"git@github.com:user/my-app.git", "my-app"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := appNameFromRepo(tt.input)
			if got != tt.want {
				t.Errorf("appNameFromRepo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTemplateArg_Empty(t *testing.T) {
	args := []string{"https://example.com/repo.git"}
	out, warn, err := resolveTemplateArg("", args)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn != "" {
		t.Errorf("warn: got %q, want empty", warn)
	}
	if len(out) != 1 || out[0] != args[0] {
		t.Errorf("args: got %v, want %v", out, args)
	}
}

func TestResolveTemplateArg_Happy(t *testing.T) {
	// Pick a template that's guaranteed to exist in the embedded manifest.
	all := templates.List()
	if len(all) == 0 {
		t.Skip("no templates in manifest")
	}
	slug := all[0].Slug

	out, warn, err := resolveTemplateArg(slug, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn != "" {
		t.Errorf("warn: got %q, want empty when no positional arg", warn)
	}
	if len(out) != 1 || !strings.Contains(out[0], ".git") {
		t.Errorf("args: got %v, want one repo URL", out)
	}
}

func TestResolveTemplateArg_Unknown(t *testing.T) {
	_, _, err := resolveTemplateArg("does-not-exist", nil)
	if !errors.Is(err, templates.ErrTemplateNotFound) {
		t.Errorf("err: got %v, want ErrTemplateNotFound", err)
	}
}

func TestResolveTemplateArg_FlagOverridesPositional(t *testing.T) {
	all := templates.List()
	if len(all) == 0 {
		t.Skip("no templates in manifest")
	}
	slug := all[0].Slug

	out, warn, err := resolveTemplateArg(slug, []string{"https://example.com/other.git"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn == "" {
		t.Error("expected warning when both flag and positional are given")
	}
	if !strings.Contains(warn, slug) {
		t.Errorf("warning should mention the slug %q: got %q", slug, warn)
	}
	if len(out) != 1 || out[0] == "https://example.com/other.git" {
		t.Errorf("args: got %v, expected the template URL to win", out)
	}
}

func TestAppNameFromDir(t *testing.T) {
	got := appNameFromDir("/home/user/projects/my-cool-app")
	want := "my-cool-app"
	if got != want {
		t.Errorf("appNameFromDir(%q) = %q, want %q", "/home/user/projects/my-cool-app", got, want)
	}
}

func TestApplySpecOverridesPort(t *testing.T) {
	fr := &detect.FrameworkResult{
		Framework:  detect.FrameworkExpress,
		Build:      "npm ci",
		Start:      "node server.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	s := &spec.Spec{Name: "x", Port: 8080, Start: "node custom.js"}

	if !applySpec(fr, s) {
		t.Errorf("applySpec returned false despite overrides")
	}

	if fr.Port != 8080 {
		t.Errorf("Port = %d, want 8080", fr.Port)
	}
	if fr.Start != "node custom.js" {
		t.Errorf("Start = %q, want override", fr.Start)
	}
	if fr.Build != "npm ci" {
		t.Errorf("Build = %q, want untouched", fr.Build)
	}
}

func TestApplySpecNilNoop(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkExpress, Port: 3000}
	if applySpec(fr, nil) {
		t.Errorf("applySpec(nil) returned true")
	}
	if fr.Port != 3000 {
		t.Errorf("Port mutated to %d", fr.Port)
	}
}

func TestApplySpec_ReportsOverridden(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkExpress, Port: 3000}
	if applySpec(fr, &spec.Spec{Name: "x"}) {
		t.Errorf("applySpec returned true for empty-override spec")
	}
	if !applySpec(fr, &spec.Spec{Name: "x", Port: 8080}) {
		t.Errorf("applySpec returned false despite Port override")
	}
}

func TestFormatDryRun(t *testing.T) {
	info := dryRunInfo{
		AppName:    "my-app",
		Framework:  "nextjs",
		Build:      "npm run build",
		Start:      "node .next/standalone/server.js",
		Port:       3000,
		DBEngine:   "postgresql",
		Dockerfile: "auto",
		Domain:     "my-app.deploy.example.com",
		Server:     "vps01",
	}

	output := formatDryRun(&info)

	if !strings.Contains(output, "my-app") {
		t.Error("expected app name in output")
	}
	if !strings.Contains(output, "nextjs") {
		t.Error("expected framework in output")
	}
	if !strings.Contains(output, "postgresql") {
		t.Error("expected database in output")
	}
	if !strings.Contains(output, "3000") {
		t.Error("expected port in output")
	}
	if !strings.Contains(output, "my-app.deploy.example.com") {
		t.Error("expected domain in output")
	}
}
