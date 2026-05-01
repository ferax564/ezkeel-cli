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

// shellQuoteForSingleQuoted is shellQuote intended for substitution
// into a string that is itself going to be wrapped in single quotes
// for a `sh -c '...'` outer command. Each literal single quote we'd
// emit is converted to the `'\''` close-escape-reopen idiom, then
// that whole already-quoted form is escaped a SECOND time so the
// outer quoting stays balanced.
//
// Without this two-level escape, `sh -c 'curl ... 'URL' && chmod'`
// parses as concatenation: the outer `'...'` ends at the first inner
// quote, the URL is unquoted (and a `&` inside it backgrounds curl),
// then the next `'...'` reopens. The agent URL must stay quoted
// across BOTH layers.
func shellQuoteForSingleQuoted(s string) string {
	inner := shellQuote(s)
	// Now escape every literal single quote inside `inner` for the
	// outer single-quoted context.
	return strings.ReplaceAll(inner, "'", `'\''`)
}

// privCmd wraps a privileged command so it runs directly when the SSH
// user is root (where `sudo` may not be installed — minimal Alpine /
// stripped Debian images, the default on Hetzner Cloud), and via
// `sudo -n` otherwise.
//
// Chained commands (foo && bar) MUST be passed wrapped in `sh -c '...'`
// by the caller. Otherwise sudo only privileges the first command in
// the chain; the && tail runs as the SSH user and a `sudo -n curl &&
// apt-get install` would silently de-escalate halfway through.
//
// Heredocs (cat <<'EOF' ... EOF) cannot be wrapped by privCmd — the
// terminator must be on a line by itself, and privCmd's single-line
// `if ...; then ...; else ...; fi` shape would put `; else ...` after
// the EOF marker on the same line, breaking the heredoc.
// caddyfileWriteCmd / caddyComposeWriteCmd build the if/else by hand
// with newlines for that reason.
func privCmd(cmd string) string {
	return fmt.Sprintf(`if [ "$(id -u)" = "0" ]; then %s; else sudo -n %s; fi`, cmd, cmd)
}

// privHeredocWrite returns a multi-line if/else that writes body to
// destPath via `tee` directly when uid is 0, and via `sudo -n tee`
// otherwise. Each `EZKEELEOF` terminator sits on its own line — the
// heredoc would not close otherwise. privCmd's single-line shape is
// unsuitable for heredocs, so this special-case helper exists.
func privHeredocWrite(destPath, body string) string {
	return fmt.Sprintf(
		`if [ "$(id -u)" = "0" ]; then
tee %s >/dev/null <<'EZKEELEOF'
%sEZKEELEOF
else
sudo -n tee %s >/dev/null <<'EZKEELEOF'
%sEZKEELEOF
fi`,
		destPath, body, destPath, body,
	)
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
//
// Uses `tee` (direct as root, via `sudo -n` otherwise) so the write
// works on minimal images that lack sudo (Hetzner default root SSH)
// AND on cloud images with passwordless sudo (ubuntu/debian on
// AWS/Vultr/Scaleway). The leading `test -f` guards short-circuit
// before tee/sudo runs on already-bootstrapped hosts, so subsequent
// runs never invoke either at all.
//
// The later chown_platform_dir step transfers ownership of /opt/ezkeel
// (including this Caddyfile) to the SSH user so subsequent `ezkeel up`
// invocations can append per-app reverse_proxy blocks via plain
// `printf >>` without sudo.
func caddyfileWriteCmd() string {
	return fmt.Sprintf(
		"test -f /opt/ezkeel/Caddyfile || test -f /opt/ezkeel/docker-compose.yml || %s",
		privHeredocWrite("/opt/ezkeel/Caddyfile", minimalCaddyfile),
	)
}

// caddyComposeWriteCmd returns the heredoc shell command that writes
// the minimal Caddy compose stack to /opt/ezkeel/compose.yml.
//
// Guarded by `test -f` so re-running bootstrap leaves any
// hand-edited compose customisations intact. `docker compose up -d`
// downstream is idempotent either way; this guard avoids surprising
// the user when they have tweaked the file.
//
// Also guarded against /opt/ezkeel/docker-compose.yml — that filename is
// owned by `ezkeel platform install` (the full Forgejo+Infisical+Caddy
// stack). On a host that has the platform installed, writing a separate
// compose.yml here AND running `docker compose up -d` would spin up a
// second minimal Caddy alongside the platform's, racing for ports 80/443
// and taking production routes down. If docker-compose.yml exists, we
// skip the write entirely; the caddy_up step does the same.
//
// Uses `tee` (direct as root, via `sudo -n` otherwise) for the same
// dual-image-shape reason as caddyfileWriteCmd.
func caddyComposeWriteCmd() string {
	return fmt.Sprintf(
		"test -f /opt/ezkeel/compose.yml || test -f /opt/ezkeel/docker-compose.yml || %s",
		privHeredocWrite("/opt/ezkeel/compose.yml", minimalCaddyCompose),
	)
}

// Steps returns the bootstrap command sequence with stable names. The
// docker_install step is included unconditionally; Run() skips it
// when docker_probe succeeds. Steps after agent_verify install a
// minimal Caddy compose stack so cmd/ezkeel/server.go's network-connect
// step has a target on a truly fresh box.
//
// Privileged steps wrap their commands in privCmd so they run directly
// when the SSH user is root (Hetzner Cloud default; minimal Alpine /
// stripped Debian images often lack `sudo` entirely) and via `sudo -n`
// otherwise — supporting both root-only minimal images AND non-root
// cloud images (ubuntu/debian on AWS/Vultr/Scaleway) with passwordless
// sudo. On a non-root host WITHOUT passwordless sudo, the step still
// fails fast with a clear "sudo: a password is required" instead of
// silently mis-installing. Read-only steps (docker_probe, agent_verify)
// deliberately stay unwrapped.
func Steps(opts Options) []Step {
	url := opts.agentURL()
	return []Step{
		// Read-only — no privilege needed.
		{Name: "docker_probe", Cmd: "docker --version && docker compose version"},

		// Privileged — install script writes to /etc, /usr/local, etc.
		// The whole download → run → cleanup chain must run as root, so
		// it's wrapped in `sh -c '...'` first. Without that wrapper,
		// privCmd would only escalate the curl call and leave `sh
		// /tmp/get-docker.sh` (the actual installer needing apt-get) and
		// `rm` running as the SSH user, breaking the install on non-root
		// hosts. We download to a tempfile rather than `curl | sh` so
		// privCmd has a single command to wrap.
		{Name: "docker_install", Cmd: privCmd(`sh -c 'curl -fsSL https://get.docker.com -o /tmp/get-docker.sh && sh /tmp/get-docker.sh && rm -f /tmp/get-docker.sh'`)},

		// Privileged — write to /usr/local/bin and chmod. Wrapped in
		// `sh -c` so the && chain stays under one privilege wrapper.
		// The URL must stay single-quoted INSIDE the sh -c body
		// (defends against `&` query separators getting parsed as
		// backgrounding operators) AND the outer body is itself
		// single-quoted for sh -c. shellQuoteForSingleQuoted handles
		// the two-level quote escaping; a plain shellQuote() would
		// terminate the outer string mid-way.
		{Name: "agent_download", Cmd: privCmd(fmt.Sprintf(
			`sh -c 'curl -fsSL -o /usr/local/bin/ezkeel-agent %s && chmod +x /usr/local/bin/ezkeel-agent'`,
			shellQuoteForSingleQuoted(url),
		))},

		// Read-only — agent binary is on PATH after the previous step.
		{Name: "agent_verify", Cmd: "ezkeel-agent --version"},

		// Privileged — docker daemon socket is root-owned by default.
		// `sh -c` wraps the inspect-or-create chain so both halves run
		// under the same privilege.
		{Name: "ezkeel_apps_network", Cmd: privCmd(`sh -c 'docker network inspect ezkeel-apps >/dev/null 2>&1 || docker network create ezkeel-apps'`)},

		// Privileged — /opt is root-owned on most distros.
		{Name: "platform_dir", Cmd: privCmd("mkdir -p /opt/ezkeel")},

		// Privileged — caddyfileWriteCmd / caddyComposeWriteCmd use
		// privHeredocWrite (root-direct or sudo -n tee) for the actual
		// writes; `test -f` guards short-circuit before either path runs
		// on idempotent re-runs.
		{Name: "caddyfile_write", Cmd: caddyfileWriteCmd()},
		{Name: "caddy_compose_write", Cmd: caddyComposeWriteCmd()},

		// Privileged — docker daemon socket access. The cd && compose-up
		// chain is wrapped in `sh -c` so privCmd escalates both halves.
		{Name: "caddy_up", Cmd: fmt.Sprintf(
			"test -f /opt/ezkeel/docker-compose.yml || %s",
			privCmd(`sh -c 'cd /opt/ezkeel && docker compose -p ezkeel up -d'`),
		)},

		// Privileged — transfer /opt/ezkeel ownership to the SSH user
		// so subsequent `ezkeel up` flows can append per-app
		// reverse_proxy blocks via plain `printf >>` without escalation.
		// `logname` returns the original SSH login name (works whether
		// connected directly as root or escalating via sudo); falls back
		// to `id -un` on hosts where logname errors (no controlling tty).
		// As root SSHing in directly, this becomes `chown root:root`
		// (no-op — files were already root-owned).
		{Name: "chown_platform_dir", Cmd: privCmd(`chown -R "$(logname 2>/dev/null || id -un):$(id -gn)" /opt/ezkeel`)},

		// Privileged — add the SSH user to the docker group so subsequent
		// SSH sessions can run plain `docker` commands (network create,
		// exec for caddy reload, load) without sudo. New group membership
		// is picked up by the NEXT login shell — each `ssh user@host cmd`
		// opens a fresh shell, so this is sufficient. NOT wrapped with
		// privCmd: as root there is no point adding "root" to the docker
		// group (root already has socket access), so the explicit
		// non-root guard handles the no-op case directly.
		{Name: "docker_group_add", Cmd: `if [ "$(id -u)" != "0" ]; then sudo -n usermod -aG docker "$(id -un)"; fi`},
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
