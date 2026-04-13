package detect

import (
	"path/filepath"
	"testing"
)

func TestManifestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps", "myapp.yaml")

	m := &AppManifest{
		Name:   "myapp",
		Repo:   "https://git.example.com/org/myapp",
		Server: "hetzner-1",
		App: AppConfig{
			Framework:  "nextjs",
			Build:      "npm run build",
			Start:      "node .next/standalone/server.js",
			Port:       3000,
			Dockerfile: "auto",
		},
		Services: map[string]ServiceConfig{
			"postgres": {
				Version:  "16",
				Database: "myapp_db",
			},
		},
		Env: map[string]string{
			"DATABASE_URL": "postgres://localhost/myapp_db",
		},
		Domain: "myapp.example.com",
	}

	if err := m.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error: %v", err)
	}

	if loaded.Name != m.Name {
		t.Errorf("Name: got %q, want %q", loaded.Name, m.Name)
	}
	if loaded.App.Framework != m.App.Framework {
		t.Errorf("App.Framework: got %q, want %q", loaded.App.Framework, m.App.Framework)
	}
	if loaded.App.Port != m.App.Port {
		t.Errorf("App.Port: got %d, want %d", loaded.App.Port, m.App.Port)
	}
	pg, ok := loaded.Services["postgres"]
	if !ok {
		t.Fatal("Services[\"postgres\"] not found")
	}
	if pg.Version != "16" {
		t.Errorf("Services[postgres].Version: got %q, want %q", pg.Version, "16")
	}
	dbURL, ok := loaded.Env["DATABASE_URL"]
	if !ok {
		t.Fatal("Env[\"DATABASE_URL\"] not found")
	}
	if dbURL != m.Env["DATABASE_URL"] {
		t.Errorf("Env[DATABASE_URL]: got %q, want %q", dbURL, m.Env["DATABASE_URL"])
	}
	if loaded.Domain != m.Domain {
		t.Errorf("Domain: got %q, want %q", loaded.Domain, m.Domain)
	}
}

func TestManifestPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	got := ManifestPath("myapp")
	want := filepath.Join(dir, "apps", "myapp.yaml")
	if got != want {
		t.Errorf("ManifestPath() = %q, want %q", got, want)
	}
}

func TestLoadManifest_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadManifest(filepath.Join(dir, "nonexistent.yaml"))
	if err == nil {
		t.Error("LoadManifest() expected error for nonexistent path, got nil")
	}
}
