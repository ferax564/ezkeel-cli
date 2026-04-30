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

// Steps returns the bootstrap command sequence with stable names. The
// docker_install step is included unconditionally; Run() skips it
// when docker_probe succeeds.
func Steps(opts Options) []Step {
	url := opts.agentURL()
	return []Step{
		{Name: "docker_probe", Cmd: "docker --version"},
		{Name: "docker_install", Cmd: "curl -fsSL https://get.docker.com | sh"},
		{Name: "agent_download", Cmd: fmt.Sprintf(
			"curl -fsSL -o /usr/local/bin/ezkeel-agent %s && chmod +x /usr/local/bin/ezkeel-agent",
			url,
		)},
		{Name: "agent_verify", Cmd: "ezkeel-agent --version"},
	}
}

// Run executes the bootstrap sequence against runner. Behaviour:
//
//  1. docker --version — if it fails, run the docker install step.
//  2. Download agent binary; -fsSL so curl dies on HTTP errors instead
//     of writing a 404 page over the binary path.
//  3. ezkeel-agent --version — catches a curl-success-but-bad-bytes
//     case where the binary path holds a 404 page.
func Run(ctx context.Context, runner Runner, opts Options) error {
	steps := Steps(opts)

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
	return nil
}
