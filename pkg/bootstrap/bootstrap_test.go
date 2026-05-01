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
	// 3 original calls (probe, download, verify) + 7 unconditional Caddy
	// install steps (network, mkdir, Caddyfile write, compose write, up,
	// chown_platform_dir, docker_group_add).
	if len(r.calls) != 10 {
		t.Fatalf("calls = %d, want 10", len(r.calls))
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
	if !strings.Contains(r.calls[8], "chown -R") {
		t.Errorf("call 8 = %q, want chown_platform_dir", r.calls[8])
	}
	if !strings.Contains(r.calls[9], "usermod -aG docker") {
		t.Errorf("call 9 = %q, want docker_group_add", r.calls[9])
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
	// 4 original calls (probe, install, download, verify) + 7 Caddy steps
	// (network, mkdir, Caddyfile write, compose write, up,
	// chown_platform_dir, docker_group_add).
	if len(r.calls) != 11 {
		t.Fatalf("calls = %d, want 11 (probe, install, download, verify, +7 caddy)", len(r.calls))
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
		"chown_platform_dir",
		"docker_group_add",
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

// TestStepsExposed_ChownAndGroupAdd guards two non-root operator steps
// added in round 9. Without `chown_platform_dir`, the SSH user can't
// `printf >> /opt/ezkeel/Caddyfile` after bootstrap (root-owned via
// `sudo -n tee` write). Without `docker_group_add`, the SSH user can't
// run `docker network create` / `docker exec` (root-owned socket). Both
// are no-ops as root.
func TestStepsExposed_ChownAndGroupAdd(t *testing.T) {
	steps := Steps(Options{AgentURL: "https://example/agent"})
	var chown, group Step
	for _, s := range steps {
		switch s.Name {
		case "chown_platform_dir":
			chown = s
		case "docker_group_add":
			group = s
		}
	}
	if !strings.Contains(chown.Cmd, "chown -R") {
		t.Errorf("chown_platform_dir must chown recursively; got: %q", chown.Cmd)
	}
	if !strings.Contains(group.Cmd, "usermod -aG docker") {
		t.Errorf("docker_group_add must add user to docker group; got: %q", group.Cmd)
	}
	if !strings.Contains(group.Cmd, `id -u`) {
		t.Errorf("docker_group_add must guard with `id -u` so root is a no-op; got: %q", group.Cmd)
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

func TestCaddyComposeWriteSkipsForPlatformInstall(t *testing.T) {
	// `ezkeel platform install` writes /opt/ezkeel/docker-compose.yml
	// (note the hyphen). On a host that has the full platform stack
	// installed, re-running `ezkeel server add` would otherwise write
	// our minimal compose.yml alongside it and `docker compose up -d`
	// would spin up a second Caddy that races for ports 80/443.
	cmd := caddyComposeWriteCmd()
	if !strings.Contains(cmd, "test -f /opt/ezkeel/docker-compose.yml") {
		t.Errorf("compose write must also guard docker-compose.yml (platform install path); got: %q", cmd)
	}
}

func TestCaddyUpStepSkipsForPlatformInstall(t *testing.T) {
	// Even if the compose write skipped (because docker-compose.yml
	// already exists), running `docker compose -p ezkeel up -d` here
	// would still bring up a minimal stack from a (now-empty) project
	// dir or interact with the platform's compose. Skip the up entirely
	// when the platform owns this directory.
	steps := Steps(Options{AgentURL: "https://example/agent"})
	var caddyUp Step
	for _, s := range steps {
		if s.Name == "caddy_up" {
			caddyUp = s
			break
		}
	}
	if caddyUp.Cmd == "" {
		t.Fatal("caddy_up step missing from Steps()")
	}
	if !strings.Contains(caddyUp.Cmd, "test -f /opt/ezkeel/docker-compose.yml") {
		t.Errorf("caddy_up step must skip when platform install owns compose; got: %q", caddyUp.Cmd)
	}
}

func TestStepsExposed_DockerProbeIncludesCompose(t *testing.T) {
	// Some Debian/Ubuntu LTS combos ship distro Docker without the
	// v2 compose plugin. If docker_probe only checked `docker --version`,
	// install would be skipped and bootstrap would later fail at
	// caddy_up. The get.docker.com install script bundles the v2
	// plugin, so triggering install on a missing plugin recovers.
	steps := Steps(Options{AgentURL: "https://example/agent"})
	var probe Step
	for _, s := range steps {
		if s.Name == "docker_probe" {
			probe = s
			break
		}
	}
	if probe.Cmd == "" {
		t.Fatal("docker_probe step missing from Steps()")
	}
	if !strings.Contains(probe.Cmd, "docker compose version") {
		t.Errorf("docker_probe must check the v2 compose plugin too; got: %q", probe.Cmd)
	}
}

func TestRunCaddyUpFails(t *testing.T) {
	r := &fakeRunner{
		resp: func(i int, cmd string) ([]byte, error) {
			// Last step (caddy_up) fails. The step is wrapped in a
			// `test -f docker-compose.yml || (...)` guard so match on
			// the docker-compose substring inside the parens.
			if strings.Contains(cmd, "docker compose -p ezkeel up -d") {
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

// TestPrivilegedStepsUseSudo guards the non-root bootstrap path. Cloud
// images (AWS, Vultr, Scaleway) default to a non-root user with
// passwordless sudo; without `sudo -n` on the privileged steps, every
// bootstrap on those providers would fail at the first /etc or /opt
// write. Read-only steps (docker_probe, agent_verify) deliberately
// avoid sudo — covered separately by TestReadOnlyStepsAvoidSudo.
func TestPrivilegedStepsUseSudo(t *testing.T) {
	steps := Steps(Options{AgentURL: "https://example/agent"})
	privileged := []string{
		"docker_install", "agent_download", "ezkeel_apps_network",
		"platform_dir", "caddyfile_write", "caddy_compose_write", "caddy_up",
		"chown_platform_dir", "docker_group_add",
	}
	privSet := make(map[string]bool, len(privileged))
	for _, n := range privileged {
		privSet[n] = true
	}
	for _, s := range steps {
		if privSet[s.Name] && !strings.Contains(s.Cmd, "sudo -n") {
			t.Errorf("step %q is privileged but command lacks `sudo -n`: %q", s.Name, s.Cmd)
		}
	}
}

// TestReadOnlyStepsAvoidSudo confirms docker_probe and agent_verify do
// not invoke sudo. They only read state; gating them behind sudo would
// reject installs on hosts that have NOPASSWD entries scoped only to
// docker/curl (a common ops practice on shared multi-tenant boxes).
func TestReadOnlyStepsAvoidSudo(t *testing.T) {
	steps := Steps(Options{AgentURL: "https://example/agent"})
	readOnly := map[string]bool{"docker_probe": true, "agent_verify": true}
	for _, s := range steps {
		if readOnly[s.Name] && strings.Contains(s.Cmd, "sudo") {
			t.Errorf("read-only step %q should not invoke sudo: %q", s.Name, s.Cmd)
		}
	}
}
