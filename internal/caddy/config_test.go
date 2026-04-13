package caddy_test

import (
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/caddy"
)

func TestGenerateCaddyfile_SingleApp(t *testing.T) {
	apps := []caddy.AppRoute{{Subdomain: "my-app", UpstreamPort: 8001}}
	content := caddy.GenerateCaddyfile("deploy.example.com", apps)

	if !strings.Contains(content, "my-app.deploy.example.com") {
		t.Errorf("expected subdomain block in Caddyfile, got:\n%s", content)
	}
	if !strings.Contains(content, "localhost:8001") {
		t.Errorf("expected upstream port in Caddyfile, got:\n%s", content)
	}
}

func TestGenerateCaddyfile_MultipleApps(t *testing.T) {
	apps := []caddy.AppRoute{
		{Subdomain: "app1", UpstreamPort: 8001},
		{Subdomain: "app2", UpstreamPort: 8002},
	}
	content := caddy.GenerateCaddyfile("deploy.example.com", apps)

	if !strings.Contains(content, "app1.deploy.example.com") {
		t.Errorf("expected app1 block in Caddyfile, got:\n%s", content)
	}
	if !strings.Contains(content, "app2.deploy.example.com") {
		t.Errorf("expected app2 block in Caddyfile, got:\n%s", content)
	}
}

func TestGenerateCaddyfile_Empty(t *testing.T) {
	content := caddy.GenerateCaddyfile("deploy.example.com", nil)
	if content == "" {
		t.Error("expected non-empty Caddyfile even with no apps")
	}
	if !strings.Contains(content, "admin localhost:2019") {
		t.Errorf("expected global options block in Caddyfile, got:\n%s", content)
	}
}

func TestNextPort(t *testing.T) {
	apps := []caddy.AppRoute{
		{UpstreamPort: 8001},
		{UpstreamPort: 8003},
	}
	got := caddy.NextAvailablePort(apps)
	if got != 8004 {
		t.Errorf("NextAvailablePort = %d, want 8004", got)
	}
}

func TestNextPort_Empty(t *testing.T) {
	got := caddy.NextAvailablePort(nil)
	if got != 8001 {
		t.Errorf("NextAvailablePort (empty) = %d, want 8001", got)
	}
}

func TestGenerateCaddyfile_SpecialCharsInName(t *testing.T) {
	apps := []caddy.AppRoute{{Subdomain: "my-app-v2", UpstreamPort: 8001}}
	content := caddy.GenerateCaddyfile("deploy.example.com", apps)

	if !strings.Contains(content, "my-app-v2.deploy.example.com") {
		t.Errorf("expected subdomain block for hyphenated name, got:\n%s", content)
	}
	if !strings.Contains(content, "localhost:8001") {
		t.Errorf("expected upstream port in Caddyfile, got:\n%s", content)
	}
}

func TestNextPort_Sequential(t *testing.T) {
	apps := []caddy.AppRoute{
		{UpstreamPort: 8001},
		{UpstreamPort: 8002},
		{UpstreamPort: 8003},
	}
	got := caddy.NextAvailablePort(apps)
	if got != 8004 {
		t.Errorf("NextAvailablePort = %d, want 8004", got)
	}
}
