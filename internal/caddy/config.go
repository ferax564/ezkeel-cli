package caddy

import (
	"fmt"
	"strings"
)

// AppRoute maps a subdomain to an upstream container port.
type AppRoute struct {
	Subdomain    string `yaml:"subdomain"`
	UpstreamPort int    `yaml:"upstream_port"`
}

// GenerateCaddyfile produces a Caddyfile with reverse proxy entries.
// wildcardDomain is e.g. "deploy.example.com"
// Each app gets "<subdomain>.deploy.example.com { reverse_proxy localhost:<port> }"
// Returns a non-empty string even with no apps (just the global options block).
func GenerateCaddyfile(wildcardDomain string, apps []AppRoute) string {
	var sb strings.Builder

	sb.WriteString("{\n")
	sb.WriteString("\tadmin localhost:2019\n")
	sb.WriteString("}\n")

	for _, app := range apps {
		sb.WriteString(fmt.Sprintf("\n%s.%s {\n", app.Subdomain, wildcardDomain))
		sb.WriteString(fmt.Sprintf("\treverse_proxy localhost:%d\n", app.UpstreamPort))
		sb.WriteString("}\n")
	}

	return sb.String()
}

// NextAvailablePort returns the next port after the highest port in use.
// Starts at 8001 if no apps exist.
func NextAvailablePort(apps []AppRoute) int {
	if len(apps) == 0 {
		return 8001
	}

	max := 0
	for _, app := range apps {
		if app.UpstreamPort > max {
			max = app.UpstreamPort
		}
	}

	return max + 1
}
