package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServerConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	srv := &Server{
		Name:   "prod",
		Host:   "192.168.1.1",
		User:   "deploy",
		SSHKey: "/home/deploy/.ssh/id_rsa",
		Domain: "example.com",
	}

	if err := SaveServer(srv); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	loaded, err := LoadServer("prod")
	if err != nil {
		t.Fatalf("LoadServer: %v", err)
	}

	if loaded.Name != srv.Name {
		t.Errorf("Name: got %q, want %q", loaded.Name, srv.Name)
	}
	if loaded.Host != srv.Host {
		t.Errorf("Host: got %q, want %q", loaded.Host, srv.Host)
	}
	if loaded.User != srv.User {
		t.Errorf("User: got %q, want %q", loaded.User, srv.User)
	}
	if loaded.SSHKey != srv.SSHKey {
		t.Errorf("SSHKey: got %q, want %q", loaded.SSHKey, srv.SSHKey)
	}
	if loaded.Domain != srv.Domain {
		t.Errorf("Domain: got %q, want %q", loaded.Domain, srv.Domain)
	}
}

func TestServerConfig_DefaultUser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	srv := &Server{
		Name:   "staging",
		Host:   "10.0.0.1",
		Domain: "staging.example.com",
		// User intentionally omitted — should default to "root"
	}

	if err := SaveServer(srv); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	loaded, err := LoadServer("staging")
	if err != nil {
		t.Fatalf("LoadServer: %v", err)
	}
	if loaded.User != "root" {
		t.Errorf("User: got %q, want %q", loaded.User, "root")
	}
}

func TestListServers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	servers := []*Server{
		{Name: "alpha", Host: "1.1.1.1", Domain: "alpha.example.com"},
		{Name: "beta", Host: "2.2.2.2", Domain: "beta.example.com"},
	}
	for _, s := range servers {
		if err := SaveServer(s); err != nil {
			t.Fatalf("SaveServer(%q): %v", s.Name, err)
		}
	}

	list, err := ListServers()
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListServers count: got %d, want 2", len(list))
	}
}

func TestLoadServer_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	_, err := LoadServer("nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent server, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message %q does not contain 'not found'", err.Error())
	}
}

func TestDefaultServer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	srv := &Server{
		Name:   "only-server",
		Host:   "9.9.9.9",
		Domain: "only.example.com",
	}
	if err := SaveServer(srv); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	def, err := DefaultServer()
	if err != nil {
		t.Fatalf("DefaultServer: %v", err)
	}
	if def.Name != srv.Name {
		t.Errorf("DefaultServer name: got %q, want %q", def.Name, srv.Name)
	}
}

func TestDefaultServer_NoServers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	_, err := DefaultServer()
	if err == nil {
		t.Fatal("expected error when no servers configured, got nil")
	}
}

func TestServersDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EZKEEL_HOME", dir)

	got := ServersDir()
	want := filepath.Join(dir, "servers")
	if got != want {
		t.Errorf("ServersDir: got %q, want %q", got, want)
	}
	if !strings.Contains(got, dir) {
		t.Errorf("ServersDir %q does not contain EZKEEL_HOME %q", got, dir)
	}
}
