package detect_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/detect"
)

func TestDetectFramework_Dockerfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM ubuntu:22.04\n"), 0644); err != nil {
		t.Fatalf("failed to write Dockerfile: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkDockerfile {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkDockerfile)
	}
	if result.Dockerfile != "Dockerfile" {
		t.Errorf("Dockerfile: got %q, want %q", result.Dockerfile, "Dockerfile")
	}
}

func TestDetectFramework_NextJS(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"next": "14.0.0", "react": "18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkNextjs {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkNextjs)
	}
	if result.Port != 3000 {
		t.Errorf("Port: got %d, want 3000", result.Port)
	}
}

func TestDetectFramework_Vite(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"devDependencies": {"vite": "5.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkVite {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkVite)
	}
}

func TestDetectFramework_Express(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"express": "4.18.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkExpress {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkExpress)
	}
	if result.Port != 3000 {
		t.Errorf("Port: got %d, want 3000", result.Port)
	}
}

func TestDetectFramework_Hono(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"hono": "4.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkHono {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkHono)
	}
}

func TestDetectFramework_FastAPI(t *testing.T) {
	dir := t.TempDir()
	req := "fastapi==0.110.0\nuvicorn==0.29.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(req), 0644); err != nil {
		t.Fatalf("failed to write requirements.txt: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkFastAPI {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkFastAPI)
	}
	if result.Port != 8000 {
		t.Errorf("Port: got %d, want 8000", result.Port)
	}
}

func TestDetectFramework_Go(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkGo {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkGo)
	}
}

func TestDetectFramework_StaticHTML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!DOCTYPE html><html><body>Hello</body></html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkStatic {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkStatic)
	}
}

func TestDetectFramework_Unknown(t *testing.T) {
	dir := t.TempDir()

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkUnknown {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkUnknown)
	}
}

func TestDetectFramework_DockerfileTakesPriority(t *testing.T) {
	dir := t.TempDir()

	// Both Dockerfile and a Next.js package.json exist
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:20\n"), 0644); err != nil {
		t.Fatalf("failed to write Dockerfile: %v", err)
	}
	pkg := `{"dependencies": {"next": "14.0.0", "react": "18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkDockerfile {
		t.Errorf("Framework: got %q, want %q (Dockerfile should take priority)", result.Framework, detect.FrameworkDockerfile)
	}
}

func TestDetectFramework_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework returned error: %v", err)
	}
	if result.Framework != detect.FrameworkUnknown {
		t.Errorf("Framework: got %q, want %q", result.Framework, detect.FrameworkUnknown)
	}
}

func TestDefaultsForPopulatesKnownFrameworks(t *testing.T) {
	cases := []struct {
		fwk      detect.Framework
		wantPort int
	}{
		{detect.FrameworkRust, 8080},
		{detect.FrameworkGo, 8080},
		{detect.FrameworkExpress, 3000},
		{detect.FrameworkFlask, 5000},
		{detect.FrameworkAstro, 4321},
		{detect.FrameworkVite, 5173},
		{detect.FrameworkStatic, 80},
		{detect.FrameworkRails, 3000},
	}
	for _, c := range cases {
		got, ok := detect.DefaultsFor(c.fwk)
		if !ok {
			t.Errorf("DefaultsFor(%q) = (_, false), want ok", c.fwk)
			continue
		}
		if got.Port != c.wantPort {
			t.Errorf("DefaultsFor(%q).Port = %d, want %d", c.fwk, got.Port, c.wantPort)
		}
		if got.Dockerfile != "auto" {
			t.Errorf("DefaultsFor(%q).Dockerfile = %q, want auto", c.fwk, got.Dockerfile)
		}
		if got.Framework != c.fwk {
			t.Errorf("DefaultsFor(%q).Framework = %q, want %q", c.fwk, got.Framework, c.fwk)
		}
	}
}

func TestDefaultsForUnknownReturnsFalse(t *testing.T) {
	if _, ok := detect.DefaultsFor(detect.FrameworkUnknown); ok {
		t.Error("DefaultsFor(FrameworkUnknown) should return false")
	}
	if _, ok := detect.DefaultsFor(detect.FrameworkDockerfile); ok {
		t.Error("DefaultsFor(FrameworkDockerfile) should return false")
	}
	if _, ok := detect.DefaultsFor(detect.Framework("not-a-framework")); ok {
		t.Error("DefaultsFor(unknown string) should return false")
	}
}

func TestDefaultsForRustMatchesDetectFramework(t *testing.T) {
	// Whatever DetectFramework returns for a Rust dir, DefaultsFor(Rust)
	// MUST match — otherwise spec-rescue would silently disagree with
	// auto-detect.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"x\"\n"), 0o644); err != nil {
		t.Fatalf("write Cargo.toml: %v", err)
	}
	detected, err := detect.DetectFramework(dir)
	if err != nil {
		t.Fatalf("DetectFramework: %v", err)
	}
	defaults, ok := detect.DefaultsFor(detect.FrameworkRust)
	if !ok {
		t.Fatal("DefaultsFor(Rust) returned false")
	}
	if detected.Build != defaults.Build || detected.Start != defaults.Start || detected.Port != defaults.Port {
		t.Errorf("Rust mismatch:\n  detected: build=%q start=%q port=%d\n  defaults: build=%q start=%q port=%d",
			detected.Build, detected.Start, detected.Port,
			defaults.Build, defaults.Start, defaults.Port)
	}
}

// TestFrameworkGoDefaultBuildIsSinglePackage guards the round-9 fix for
// multi-package Go modules. `go build -o <file> ./...` errors with
// "cannot write multiple packages to non-directory" whenever a repo has
// internal/, cmd/X/, or any other non-root packages — i.e. nearly every
// real-world Go service. The canonical default builds the current dir
// only; repos with main at ./cmd/<name> override Build in ezkeel.yaml.
func TestFrameworkGoDefaultBuildIsSinglePackage(t *testing.T) {
	fr, ok := detect.DefaultsFor(detect.FrameworkGo)
	if !ok {
		t.Fatal("DefaultsFor(FrameworkGo) returned false")
	}
	if strings.Contains(fr.Build, "./...") {
		t.Errorf("Default Go build must not use ./... with -o (fails for multi-package modules); got: %q", fr.Build)
	}
	if !strings.Contains(fr.Build, "-o /app/app") {
		t.Errorf("Default Go build must produce /app/app for the runner stage COPY; got: %q", fr.Build)
	}
}
