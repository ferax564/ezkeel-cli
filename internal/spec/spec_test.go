package spec

import (
	"os"
	"path/filepath"
	"strings"
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

// TestLoadFromDirMissing asserts the missing-spec case returns (nil, nil)
// so callers can branch on `s != nil` without an err == fs.ErrNotExist
// dance. `ezkeel up <repo-url>` relies on this — a cloned repo with no
// ezkeel.yaml is the common case, not an error.
func TestLoadFromDirMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil for missing file, got %+v", s)
	}
}

// TestLoadFromDirNoWalkUp pins down the no-walk-up policy. If ezkeel up
// clones a repo to /tmp/ezkeel-clone-XXX and that repo has no spec,
// LoadFromDir on the clone dir MUST NOT find a /tmp/ezkeel.yaml or any
// other spec from a parent directory — that would silently apply an
// unrelated config to the deploy.
func TestLoadFromDirNoWalkUp(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "ezkeel.yaml"), []byte("# spec: ezkeel/v1\nname: leak\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sub := filepath.Join(parent, "child")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	s, err := LoadFromDir(sub)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s != nil {
		t.Errorf("LoadFromDir must NOT walk up; got spec from parent: %+v", s)
	}
}

func TestLoadFromDirHonorsLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ezkeel.yaml"), []byte("# spec: ezkeel/v1\nname: local\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s == nil || s.Name != "local" {
		t.Errorf("got %+v, want spec with name=local", s)
	}
}

// TestLoadRejectsUnsafeName asserts spec.Name is validated at parse time.
// The name is interpolated into Docker tags, DNS labels, and remote
// shell command strings (Caddyfile route writers), so unsafe characters
// must fail loud at Load() rather than escape downstream.
func TestLoadRejectsUnsafeName(t *testing.T) {
	cases := []string{
		"Evil",                  // uppercase
		"evil one",              // space
		"evil;rm",               // semicolon
		`evil"`,                 // quote
		"-leading-dash",         // leading dash
		"trailing-dash-",        // trailing dash
		"ev/il",                 // slash
		strings.Repeat("a", 64), // too long
	}
	for _, name := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "ezkeel.yaml")
		body := "# spec: ezkeel/v1\nname: " + name + "\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("Load(%q) expected error, got nil", name)
		}
	}
}

func TestLoadAcceptsValidName(t *testing.T) {
	cases := []string{"a", "my-app", "app1", "x-y-z", strings.Repeat("a", 63)}
	for _, name := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "ezkeel.yaml")
		body := "# spec: ezkeel/v1\nname: " + name + "\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
		if _, err := Load(path); err != nil {
			t.Errorf("Load(%q) unexpected error: %v", name, err)
		}
	}
}
