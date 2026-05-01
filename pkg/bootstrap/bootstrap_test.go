package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []string
	resp  func(call int, cmd string) ([]byte, error)
}

func (f *fakeRunner) Run(_ context.Context, cmd string) ([]byte, error) {
	f.calls = append(f.calls, cmd)
	if f.resp == nil {
		return nil, nil
	}
	return f.resp(len(f.calls)-1, cmd)
}

func TestRunHappyPath(t *testing.T) {
	r := &fakeRunner{}
	err := Run(context.Background(), r, Options{AgentURL: "https://example/agent"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 3 original calls (probe, download, verify) + 5 unconditional Caddy
	// install steps (network, mkdir, Caddyfile write, compose write, up).
	if len(r.calls) != 8 {
		t.Fatalf("calls = %d, want 8", len(r.calls))
	}
	if !strings.HasPrefix(r.calls[0], "docker --version") {
		t.Errorf("call 0 = %q", r.calls[0])
	}
	if !strings.Contains(r.calls[1], "https://example/agent") {
		t.Errorf("call 1 missing agent url: %q", r.calls[1])
	}
	if r.calls[2] != "ezkeel-agent --version" {
		t.Errorf("call 2 = %q", r.calls[2])
	}
	if !strings.Contains(r.calls[7], "docker compose -p ezkeel up -d") {
		t.Errorf("call 7 = %q, want caddy_up", r.calls[7])
	}
}

func TestRunDockerMissingTriggersInstall(t *testing.T) {
	r := &fakeRunner{
		resp: func(i int, cmd string) ([]byte, error) {
			if i == 0 {
				return nil, errors.New("not found")
			}
			return nil, nil
		},
	}
	err := Run(context.Background(), r, Options{AgentURL: "https://example/agent"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 4 original calls (probe, install, download, verify) + 5 Caddy steps.
	if len(r.calls) != 9 {
		t.Fatalf("calls = %d, want 9 (probe, install, download, verify, +5 caddy)", len(r.calls))
	}
	if !strings.Contains(r.calls[1], "get.docker.com") {
		t.Errorf("call 1 = %q", r.calls[1])
	}
}

func TestRunDockerInstallFails(t *testing.T) {
	r := &fakeRunner{
		resp: func(i int, cmd string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	err := Run(context.Background(), r, Options{AgentURL: "https://example/agent"})
	if err == nil || !strings.Contains(err.Error(), "docker install") {
		t.Fatalf("err = %v", err)
	}
	if len(r.calls) != 2 {
		t.Errorf("calls = %d, want 2 (probe then install)", len(r.calls))
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err missing wrapped underlying: %v", err)
	}
}

func TestRunAgentVersionFails(t *testing.T) {
	r := &fakeRunner{
		resp: func(i int, cmd string) ([]byte, error) {
			if i == 2 {
				return []byte("not a binary"), errors.New("exec format")
			}
			return nil, nil
		},
	}
	err := Run(context.Background(), r, Options{AgentURL: "https://example/agent"})
	if err == nil || !strings.Contains(err.Error(), "agent --version") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "exec format") {
		t.Errorf("err missing underlying %q: %v", "exec format", err)
	}
	if !strings.Contains(err.Error(), "not a binary") {
		t.Errorf("err missing captured stderr %q: %v", "not a binary", err)
	}
	if len(r.calls) != 3 {
		t.Errorf("calls = %d, want 3 (probe, download, verify)", len(r.calls))
	}
}

func TestSSHArgsDefaults(t *testing.T) {
	got := sshArgs("1.2.3.4", "", 0, "", nil, "echo hi")
	want := []string{
		"-p", "22",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"root@1.2.3.4",
		"echo hi",
	}
	if !sliceEq(got, want) {
		t.Errorf("sshArgs defaults =\n  got  %v\n  want %v", got, want)
	}
}

func TestSSHArgsExplicit(t *testing.T) {
	got := sshArgs("h", "alice", 2222, "/k", nil, "id")
	want := []string{
		"-i", "/k",
		"-p", "2222",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"alice@h",
		"id",
	}
	if !sliceEq(got, want) {
		t.Errorf("sshArgs explicit =\n  got  %v\n  want %v", got, want)
	}
}

func TestSSHArgsHostKeyOptsOverrideDefaults(t *testing.T) {
	// Caller wants BatchMode=no for an interactive bootstrap. Per
	// ssh_config(5) "first obtained value wins", the caller's -o
	// must appear in argv BEFORE the runner's default for the
	// override to take effect.
	got := sshArgs("h", "u", 22, "", []string{"-o", "BatchMode=no"}, "id")

	overridePos := -1
	defaultPos := -1
	for i := 0; i < len(got)-1; i++ {
		if got[i] == "-o" && got[i+1] == "BatchMode=no" {
			overridePos = i
		}
		if got[i] == "-o" && got[i+1] == "BatchMode=yes" {
			defaultPos = i
		}
	}
	if overridePos == -1 {
		t.Fatalf("HostKeyOpts -o BatchMode=no missing from argv: %v", got)
	}
	if defaultPos == -1 {
		t.Fatalf("default -o BatchMode=yes missing from argv: %v", got)
	}
	if overridePos >= defaultPos {
		t.Errorf("override @%d must precede default @%d (first-occurrence-wins): %v", overridePos, defaultPos, got)
	}
}

func TestAliasArgs(t *testing.T) {
	got := aliasArgs("my-alias", "uptime")
	want := []string{
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"my-alias", "uptime",
	}
	if !sliceEq(got, want) {
		t.Errorf("aliasArgs =\n  got  %v\n  want %v", got, want)
	}
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStepsExposed(t *testing.T) {
	steps := Steps(Options{AgentURL: "https://example/agent"})
	want := []string{
		"docker_probe",
		"docker_install",
		"agent_download",
		"agent_verify",
		"ezkeel_apps_network",
		"platform_dir",
		"caddyfile_write",
		"caddy_compose_write",
		"caddy_up",
	}
	if len(steps) != len(want) {
		t.Fatalf("len = %d, want %d", len(steps), len(want))
	}
	for i, s := range steps {
		if s.Name != want[i] {
			t.Errorf("steps[%d].Name = %q, want %q", i, s.Name, want[i])
		}
	}
}

func TestMinimalCaddyfileAllowsAdminReload(t *testing.T) {
	// `docker exec ezkeel-caddy-1 caddy reload --config /etc/caddy/Caddyfile`
	// (cmd/ezkeel/server.go) talks to Caddy's admin endpoint. If the
	// Caddyfile disables admin, reload silently fails and freshly-added
	// per-app routes never load — the public domain 404s.
	if strings.Contains(minimalCaddyfile, "admin off") {
		t.Fatalf("minimalCaddyfile must not disable admin API; caddy reload depends on it")
	}
}

func TestCaddyfileWriteIsIdempotent(t *testing.T) {
	// `ezkeel server add` is now bootstrap-by-default. Re-running it
	// against an already-deployed host (e.g. to update the domain)
	// must NOT clobber the per-app reverse_proxy blocks that
	// `ezkeel up` has appended to /opt/ezkeel/Caddyfile. Guard via
	// `test -f` so subsequent runs are a no-op.
	cmd := caddyfileWriteCmd()
	if !strings.HasPrefix(cmd, "test -f /opt/ezkeel/Caddyfile ||") {
		t.Errorf("caddyfile write must be idempotent (test -f guard); got: %q", cmd)
	}
}

func TestCaddyComposeWriteIsIdempotent(t *testing.T) {
	// Same rationale as TestCaddyfileWriteIsIdempotent — bootstrap
	// is default-on, so we must not overwrite a hand-edited
	// compose.yml on every re-run.
	cmd := caddyComposeWriteCmd()
	if !strings.HasPrefix(cmd, "test -f /opt/ezkeel/compose.yml ||") {
		t.Errorf("compose write must be idempotent (test -f guard); got: %q", cmd)
	}
}

func TestShellQuoteSimple(t *testing.T) {
	got := shellQuote("https://example.com/foo?a=1&b=2")
	want := `'https://example.com/foo?a=1&b=2'`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellQuoteWithSingleQuote(t *testing.T) {
	got := shellQuote(`a'b`)
	want := `'a'\''b'`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStepsExposed_AgentDownloadIsQuoted(t *testing.T) {
	// AgentURL with `&` query params must be single-quoted so the remote
	// login shell doesn't treat `&` as a backgrounding operator and split
	// curl off from chmod.
	steps := Steps(Options{AgentURL: "https://x.example/path?a=1&b=2"})
	var dl Step
	for _, s := range steps {
		if s.Name == "agent_download" {
			dl = s
			break
		}
	}
	if dl.Cmd == "" {
		t.Fatal("agent_download step missing from Steps()")
	}
	if !strings.Contains(dl.Cmd, `'https://x.example/path?a=1&b=2'`) {
		t.Errorf("agent_download URL must be single-quoted; got: %q", dl.Cmd)
	}
}

func TestRunCaddyUpFails(t *testing.T) {
	r := &fakeRunner{
		resp: func(i int, cmd string) ([]byte, error) {
			// Last step (caddy_up) fails.
			if strings.HasPrefix(cmd, "cd /opt/ezkeel && docker compose") {
				return []byte("compose error"), errors.New("exit 1")
			}
			return nil, nil
		},
	}
	err := Run(context.Background(), r, Options{AgentURL: "https://example/agent"})
	if err == nil || !strings.Contains(err.Error(), "caddy_up") {
		t.Fatalf("err = %v", err)
	}
}
