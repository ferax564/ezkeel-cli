package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidV1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ezkeel.yaml")
	body := `# spec: ezkeel/v1
name: my-app
framework: nodejs
port: 3000
services:
  db:
    engine: postgres
    version: "16"
runtime: docker
sandbox: false
env:
  - NODE_ENV
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != "my-app" {
		t.Errorf("Name = %q, want %q", got.Name, "my-app")
	}
	if got.Framework != "nodejs" {
		t.Errorf("Framework = %q, want %q", got.Framework, "nodejs")
	}
	if got.Port != 3000 {
		t.Errorf("Port = %d, want 3000", got.Port)
	}
	if got.Services["db"].Engine != "postgres" {
		t.Errorf("services.db.engine = %q", got.Services["db"].Engine)
	}
	if got.Runtime != "docker" {
		t.Errorf("Runtime = %q", got.Runtime)
	}
	if got.Sandbox {
		t.Errorf("Sandbox = true, want false")
	}
	if len(got.Env) != 1 || got.Env[0] != "NODE_ENV" {
		t.Errorf("Env = %v", got.Env)
	}
}

func TestLoadMissingStampRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ezkeel.yaml")
	if err := os.WriteFile(path, []byte("name: x\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("Load expected error for missing stamp")
	}
}

func TestLoadUnknownVersionRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ezkeel.yaml")
	if err := os.WriteFile(path, []byte("# spec: ezkeel/v9\nname: x\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("Load expected error for unknown version")
	}
}

func TestFindWalksUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ezkeel.yaml"), []byte("# spec: ezkeel/v1\nname: x\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got, err := Find(sub)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != filepath.Join(dir, "ezkeel.yaml") {
		t.Errorf("Find = %q, want %q", got, filepath.Join(dir, "ezkeel.yaml"))
	}
}
