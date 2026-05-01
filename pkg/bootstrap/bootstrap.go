// Package bootstrap installs Docker and ezkeel-agent on a remote host
// over an injectable Runner. The CLI uses an SSH-backed Runner; the
// dashboard uses its own runner that writes status to the database.
//
// All steps are idempotent: re-running on a healthy host is a no-op.
package bootstrap

import (
	"context"
	"fmt"
	"strings"
)

// DefaultAgentURL is the release asset fetched when Options.AgentURL is empty.
const DefaultAgentURL = "https://github.com/ferax564/ezkeel-cli/releases/latest/download/ezkeel-agent-linux-amd64"

// minimalCaddyfile is the deploy-target Caddyfile. Empty of routes by
// design — `ezkeel up <repo>` later writes per-app reverse_proxy entries
// via cmd/ezkeel/server.go's addCaddyRoute().
//
// Crucially this MUST NOT disable the admin API. After appending a
// per-app reverse_proxy block, cmd/ezkeel/server.go runs
// `docker exec ezkeel-caddy-1 caddy reload --config /etc/caddy/Caddyfile`,
// which talks to the in-container admin endpoint (default
// localhost:2019). With `admin off`, reload silently fails and the
// freshly-routed app would 404 on its public domain. Caddy's default
// admin only listens inside the container, so leaving it on does not
// expose anything to the public network.
const minimalCaddyfile = `# Managed by ezkeel server add. Per-app routes are
# appended below by ezkeel up.
`

// minimalCaddyCompose runs Caddy on the external ezkeel-apps network with
// host ports 80/443. The compose project name is "ezkeel" so the resulting
// container is "ezkeel-caddy-1" (matches cmd/ezkeel/server.go's
// caddyContainer constant).
const minimalCaddyCompose = `name: ezkeel
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
    networks:
      - ezkeel-apps
    restart: unless-stopped

volumes:
  caddy_data:

networks:
  ezkeel-apps:
    external: true
`

// Runner executes a shell command on the target host and returns
// combined stdout+stderr. Implementations are responsible for
// transport (ssh, exec.CommandContext, etc.).
type Runner interface {
	Run(ctx context.Context, cmd string) ([]byte, error)
}

// Options tunes the bootstrap. Zero values are sensible defaults.
type Options struct {
	AgentURL string // override the released agent binary URL
}

// Step is one named shell command in the bootstrap sequence. Exposed
// so callers (e.g. the dashboard) can render progress per step.
type Step struct {
	Name string
	Cmd  string
}

func (o Options) agentURL() string {
	if o.AgentURL != "" {
		return o.AgentURL
	}
	return DefaultAgentURL
}

// shellQuote single-quotes s for safe substitution into a remote shell
// command string. Single quotes inside s are escaped via the '\''
// idiom (close quote, escaped quote, reopen quote).
//
// Rationale: AgentURL may carry presigned-asset query strings whose
// `&` characters would otherwise be parsed by the remote login shell
// as backgrounding operators, splitting curl off from the rest of the
// command and racing chmod against an unfinished download.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// caddyfileWriteCmd returns the heredoc shell command that writes the
// minimal Caddyfile to /opt/ezkeel/Caddyfile. Single-quoted heredoc
// delimiter prevents shell expansion of the body.
//
// Guarded by `test -f` so re-running `ezkeel server add` against an
// already-bootstrapped host (e.g. to update the platform domain) does
// not clobber the per-app reverse_proxy blocks that `ezkeel up`
// appended after the initial bootstrap. With `--bootstrap` default-on
// this is a hard requirement: an unconditional cat would wipe every
// running app's route on the next reload.
func caddyfileWriteCmd() string {
	return fmt.Sprintf(
		"test -f /opt/ezkeel/Caddyfile || cat > /opt/ezkeel/Caddyfile <<'EZKEELEOF'\n%sEZKEELEOF",
		minimalCaddyfile,
	)
}

// caddyComposeWriteCmd returns the heredoc shell command that writes
// the minimal Caddy compose stack to /opt/ezkeel/compose.yml.
//
// Guarded by `test -f` so re-running bootstrap leaves any
// hand-edited compose customisations intact. `docker compose up -d`
// downstream is idempotent either way; this guard avoids surprising
// the user when they have tweaked the file.
func caddyComposeWriteCmd() string {
	return fmt.Sprintf(
		"test -f /opt/ezkeel/compose.yml || cat > /opt/ezkeel/compose.yml <<'EZKEELEOF'\n%sEZKEELEOF",
		minimalCaddyCompose,
	)
}

// Steps returns the bootstrap command sequence with stable names. The
// docker_install step is included unconditionally; Run() skips it
// when docker_probe succeeds. Steps after agent_verify install a
// minimal Caddy compose stack so cmd/ezkeel/server.go's network-connect
// step has a target on a truly fresh box.
func Steps(opts Options) []Step {
	url := opts.agentURL()
	return []Step{
		{Name: "docker_probe", Cmd: "docker --version"},
		{Name: "docker_install", Cmd: "curl -fsSL https://get.docker.com | sh"},
		{Name: "agent_download", Cmd: fmt.Sprintf(
			"curl -fsSL -o /usr/local/bin/ezkeel-agent %s && chmod +x /usr/local/bin/ezkeel-agent",
			shellQuote(url),
		)},
		{Name: "agent_verify", Cmd: "ezkeel-agent --version"},
		{Name: "ezkeel_apps_network", Cmd: "docker network inspect ezkeel-apps >/dev/null 2>&1 || docker network create ezkeel-apps"},
		{Name: "platform_dir", Cmd: "mkdir -p /opt/ezkeel"},
		{Name: "caddyfile_write", Cmd: caddyfileWriteCmd()},
		{Name: "caddy_compose_write", Cmd: caddyComposeWriteCmd()},
		{Name: "caddy_up", Cmd: "cd /opt/ezkeel && docker compose -p ezkeel up -d"},
	}
}

// Run executes the bootstrap sequence against runner. Behaviour:
//
//  1. docker --version — if it fails, run the docker install step.
//  2. Download agent binary; -fsSL so curl dies on HTTP errors instead
//     of writing a 404 page over the binary path.
//  3. ezkeel-agent --version — catches a curl-success-but-bad-bytes
//     case where the binary path holds a 404 page.
//  4. Steps 4+ install a minimal Caddy compose stack at /opt/ezkeel
//     (network, dir, files, compose up). They run unconditionally and
//     surface real errors — they're idempotent so re-running on a
//     healthy box is a no-op.
func Run(ctx context.Context, runner Runner, opts Options) error {
	steps := Steps(opts)

	// docker_probe → docker_install (conditional) → agent_download → agent_verify
	if _, err := runner.Run(ctx, steps[0].Cmd); err != nil {
		if _, installErr := runner.Run(ctx, steps[1].Cmd); installErr != nil {
			return fmt.Errorf("docker install: %w", installErr)
		}
	}

	if _, err := runner.Run(ctx, steps[2].Cmd); err != nil {
		return fmt.Errorf("agent download: %w", err)
	}

	out, err := runner.Run(ctx, steps[3].Cmd)
	if err != nil {
		return fmt.Errorf("agent --version: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Steps 4+ run unconditionally and surface real errors.
	for i := 4; i < len(steps); i++ {
		if _, err := runner.Run(ctx, steps[i].Cmd); err != nil {
			return fmt.Errorf("%s: %w", steps[i].Name, err)
		}
	}
	return nil
}
