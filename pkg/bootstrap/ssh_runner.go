package bootstrap

import (
	"context"
	"fmt"
	"os/exec"
)

// SSHRunner shells out via ssh(1). HostKeyOpts is appended BEFORE the
// runner's defaults so caller-supplied -o flags win on duplicate keys
// (ssh first-occurrence-wins semantics, per ssh_config(5)). Use
// HostKeyOpts to override defaults like BatchMode, ConnectTimeout, or
// StrictHostKeyChecking=accept-new, or to pin a known_hosts file.
//
// Runner defaults (overridable via HostKeyOpts):
//   - BatchMode=yes
//   - ConnectTimeout=10
//   - StrictHostKeyChecking=accept-new
type SSHRunner struct {
	Host        string
	User        string
	Port        int    // 0 → 22
	KeyFile     string // empty → ssh agent / default identity
	HostKeyOpts []string
}

// AliasRunner shells out via ssh(1) using a host alias from
// ~/.ssh/config. ssh resolves HostName/Port/User/IdentityFile from
// config, so we pass the alias as the only target argument.
//
// Only ConnectTimeout is enforced — BatchMode is left to ~/.ssh/config
// so an interactive bootstrap (passphrase prompt) is possible if the
// operator wants it.
type AliasRunner struct {
	Alias string
}

// Run executes cmd on the alias target.
func (r AliasRunner) Run(ctx context.Context, cmd string) ([]byte, error) {
	args := aliasArgs(r.Alias, cmd)
	// #nosec G204 -- alias is operator-controlled config; cmd from Steps() allow-list.
	return exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
}

// Run executes cmd on the remote host. The command is passed as a
// single argv entry to ssh; sshd will hand it to the remote login
// shell. We do NOT shell-quote here — callers compose Cmd strings
// from a fixed allow-listed set in Steps().
func (r SSHRunner) Run(ctx context.Context, cmd string) ([]byte, error) {
	args := sshArgs(r.Host, r.User, r.Port, r.KeyFile, r.HostKeyOpts, cmd)
	// #nosec G204 -- ssh argv is composed from operator-controlled
	// fields (host/user/port/keyFile) and a fixed cmd allow-list in
	// bootstrap.Steps(); not user-supplied shell input.
	return exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
}

// sshArgs builds the argv slice for ssh(1). HostKeyOpts is emitted
// before the runner's defaults so the caller's -o entries are the
// first-obtained values and win (ssh_config(5) first-occurrence-wins).
func sshArgs(host, user string, port int, keyFile string, hostKeyOpts []string, cmd string) []string {
	if user == "" {
		user = "root"
	}
	if port == 0 {
		port = 22
	}
	args := make([]string, 0, 8+len(hostKeyOpts))
	if keyFile != "" {
		args = append(args, "-i", keyFile)
	}
	args = append(args, "-p", fmt.Sprintf("%d", port))
	// Caller overrides FIRST (ssh first-occurrence-wins).
	args = append(args, hostKeyOpts...)
	// Runner defaults LAST so they only apply when the caller did not
	// supply the same key.
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
	)
	args = append(args, user+"@"+host, cmd)
	return args
}

// aliasArgs builds the argv slice for an alias-based ssh invocation.
// BatchMode is intentionally NOT forced — the operator's ~/.ssh/config
// can request an interactive flow (passphrase) if needed.
func aliasArgs(alias, cmd string) []string {
	return []string{
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		alias,
		cmd,
	}
}
