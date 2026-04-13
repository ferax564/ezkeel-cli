package detect_test

import (
	"os"
	"path/filepath"
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
