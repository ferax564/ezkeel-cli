package main

import (
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

// fakeRunner records every command and answers via the respond func.
// A nil respond means every command succeeds with empty output.
type fakeRunner struct {
	calls   []string
	respond func(cmd string) ([]byte, error)
}

func (f *fakeRunner) exec(name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, cmd)
	if f.respond == nil {
		return nil, nil
	}
	return f.respond(cmd)
}

func (f *fakeRunner) Output(name string, args ...string) ([]byte, error) {
	return f.exec(name, args...)
}

func (f *fakeRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	return f.exec(name, args...)
}

func (f *fakeRunner) Run(name string, args ...string) error {
	_, err := f.exec(name, args...)
	return err
}

// called reports whether any recorded command contains substr.
func (f *fakeRunner) called(substr string) bool {
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

type exitErr struct{ msg string }

func (e exitErr) Error() string { return e.msg }

// --- dispatch / protocol versioning ---

func TestDispatch_RejectsNewerProtocol(t *testing.T) {
	r := &fakeRunner{}
	resp := dispatch(r, &agent.Request{
		ProtocolVersion: agent.CurrentProtocolVersion + 1,
		Type:            agent.CmdStatus,
	})
	if resp.OK {
		t.Fatal("request from a newer protocol must be rejected")
	}
	if !strings.Contains(resp.Error, "update the agent") {
		t.Errorf("error should tell the caller to update the agent: %q", resp.Error)
	}
	if len(r.calls) != 0 {
		t.Errorf("rejected request must not execute anything; ran %v", r.calls)
	}
}

func TestDispatch_AcceptsOlderAndCurrentProtocol(t *testing.T) {
	for _, v := range []int{0, agent.CurrentProtocolVersion} {
		r := &fakeRunner{}
		resp := dispatch(r, &agent.Request{ProtocolVersion: v, Type: agent.CmdStatus})
		if !resp.OK {
			t.Errorf("protocol version %d must be accepted: %s", v, resp.Error)
		}
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	resp := dispatch(&fakeRunner{}, &agent.Request{Type: "nonexistent"})
	if resp.OK || !strings.Contains(resp.Error, "unknown command type") {
		t.Errorf("resp = %+v", resp)
	}
}

func TestDispatch_MissingPayloads(t *testing.T) {
	cases := []agent.CmdType{
		agent.CmdDeploy, agent.CmdStop, agent.CmdLogs,
		agent.CmdDBCreate, agent.CmdDBMigrate, agent.CmdDBBackup, agent.CmdRollback,
	}
	for _, typ := range cases {
		r := &fakeRunner{}
		resp := dispatch(r, &agent.Request{Type: typ})
		if resp.OK {
			t.Errorf("%s with nil payload must fail", typ)
		}
		if len(r.calls) != 0 {
			t.Errorf("%s with nil payload must not execute anything; ran %v", typ, r.calls)
		}
	}
}

func TestDispatch_Version(t *testing.T) {
	r := &fakeRunner{}
	resp := dispatch(r, &agent.Request{Type: agent.CmdVersion})
	if !resp.OK {
		t.Fatalf("version probe failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Message, "ezkeel-agent v") {
		t.Errorf("version message = %q", resp.Message)
	}
	if len(r.calls) != 0 {
		t.Errorf("version probe must be side-effect free; ran %v", r.calls)
	}
}

// --- deploy ---

func TestHandleDeploy_Success(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker inspect") {
				return []byte("old-image:abc\n"), nil
			}
			return nil, nil
		},
	}
	resp := handleDeploy(r, &agent.DeployRequest{AppName: "myapp", ImageTag: "myapp:new", Port: 3000})
	if !resp.OK {
		t.Fatalf("deploy failed: %s", resp.Error)
	}
	if !r.called("docker tag old-image:abc myapp:prev") {
		t.Errorf("previous image was not tagged :prev; calls: %v", r.calls)
	}
	if !r.called("docker stop ezkeel-myapp") || !r.called("docker rm ezkeel-myapp") {
		t.Errorf("old container was not stopped/removed; calls: %v", r.calls)
	}
	if !r.called("docker run -d --name ezkeel-myapp") {
		t.Errorf("new container was not started; calls: %v", r.calls)
	}
	if strings.Contains(resp.Message, "warning") {
		t.Errorf("clean deploy must not carry a warning: %q", resp.Message)
	}
}

func TestHandleDeploy_TagFailureSurfacedInMessage(t *testing.T) {
	// The :prev tag is what rollback (and the control plane's
	// auto-rollback) depends on. A failed tag must not abort the deploy
	// but MUST be visible to the operator.
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			switch {
			case strings.HasPrefix(cmd, "docker inspect"):
				return []byte("old-image:abc\n"), nil
			case strings.HasPrefix(cmd, "docker tag"):
				return []byte("no space left on device"), exitErr{"exit status 1"}
			default:
				return nil, nil
			}
		},
	}
	resp := handleDeploy(r, &agent.DeployRequest{AppName: "myapp", ImageTag: "myapp:new"})
	if !resp.OK {
		t.Fatalf("deploy should still succeed when tagging fails: %s", resp.Error)
	}
	if !strings.Contains(resp.Message, "rollback will not be available") {
		t.Errorf("tag failure must be surfaced in the message: %q", resp.Message)
	}
	if !strings.Contains(resp.Message, "no space left on device") {
		t.Errorf("message should carry the docker error detail: %q", resp.Message)
	}
}

func TestHandleDeploy_NoExistingContainerSkipsTag(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker inspect") {
				return []byte("No such object"), exitErr{"exit status 1"}
			}
			return nil, nil
		},
	}
	resp := handleDeploy(r, &agent.DeployRequest{AppName: "myapp", ImageTag: "myapp:new"})
	if !resp.OK {
		t.Fatalf("first deploy failed: %s", resp.Error)
	}
	if r.called("docker tag") {
		t.Errorf("first deploy must not tag a :prev image; calls: %v", r.calls)
	}
}

func TestHandleDeploy_RunFailure(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker run") {
				return []byte("port already allocated"), exitErr{"exit status 125"}
			}
			return nil, nil
		},
	}
	resp := handleDeploy(r, &agent.DeployRequest{AppName: "myapp", ImageTag: "myapp:new"})
	if resp.OK {
		t.Fatal("deploy must fail when docker run fails")
	}
	if !strings.Contains(resp.Error, "port already allocated") {
		t.Errorf("error should carry docker output: %q", resp.Error)
	}
}

// --- stop ---

func TestHandleStop_Success(t *testing.T) {
	r := &fakeRunner{}
	resp := handleStop(r, &agent.StopRequest{AppName: "myapp"})
	if !resp.OK {
		t.Fatalf("stop failed: %s", resp.Error)
	}
	if !r.called("docker stop ezkeel-myapp") || !r.called("docker rm ezkeel-myapp") {
		t.Errorf("calls: %v", r.calls)
	}
}

func TestHandleStop_DockerFailure(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker stop") {
				return []byte("No such container"), exitErr{"exit status 1"}
			}
			return nil, nil
		},
	}
	resp := handleStop(r, &agent.StopRequest{AppName: "myapp"})
	if resp.OK {
		t.Fatal("stop must fail when docker stop fails")
	}
	if !strings.Contains(resp.Error, "No such container") {
		t.Errorf("error should carry docker output: %q", resp.Error)
	}
}

// --- rollback ---

func TestHandleRollback_Success(t *testing.T) {
	r := &fakeRunner{}
	resp := handleRollback(r, &agent.RollbackRequest{AppName: "myapp", Port: 3000})
	if !resp.OK {
		t.Fatalf("rollback failed: %s", resp.Error)
	}
	if !r.called("docker image inspect myapp:prev") {
		t.Errorf("rollback must check the :prev image first; calls: %v", r.calls)
	}
	startedPrev := false
	for _, c := range r.calls {
		if strings.HasPrefix(c, "docker run") && strings.HasSuffix(c, "myapp:prev") {
			startedPrev = true
		}
	}
	if !startedPrev {
		t.Errorf("rollback must start the :prev image; calls: %v", r.calls)
	}
}

func TestHandleRollback_NoPrevImage(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker image inspect") {
				return nil, exitErr{"exit status 1"}
			}
			return nil, nil
		},
	}
	resp := handleRollback(r, &agent.RollbackRequest{AppName: "myapp"})
	if resp.OK {
		t.Fatal("rollback without a :prev image must fail")
	}
	if !strings.Contains(resp.Error, "nothing to roll back to") {
		t.Errorf("error = %q", resp.Error)
	}
	if r.called("docker stop") {
		t.Errorf("the running container must not be touched when there is no rollback target; calls: %v", r.calls)
	}
}

func TestHandleRollback_RunFailure(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker run") {
				return []byte("boom"), exitErr{"exit status 125"}
			}
			return nil, nil
		},
	}
	resp := handleRollback(r, &agent.RollbackRequest{AppName: "myapp"})
	if resp.OK {
		t.Fatal("rollback must fail when docker run fails")
	}
	if !strings.Contains(resp.Error, "rollback failed") {
		t.Errorf("error = %q", resp.Error)
	}
}

// --- status / logs ---

func TestHandleStatus_ParsesContainers(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			return []byte("ezkeel-api\tUp 2 hours\t127.0.0.1:3000->3000/tcp\nezkeel-web\tExited (0) 5 minutes ago\t\n"), nil
		},
	}
	resp := handleStatus(r)
	if !resp.OK {
		t.Fatalf("status failed: %s", resp.Error)
	}
	if len(resp.Apps) != 2 {
		t.Fatalf("apps = %d, want 2", len(resp.Apps))
	}
	if resp.Apps[0].Name != "api" || resp.Apps[0].Status != "running" || resp.Apps[0].Port != 3000 {
		t.Errorf("apps[0] = %+v", resp.Apps[0])
	}
	if resp.Apps[1].Name != "web" || resp.Apps[1].Status != "stopped" {
		t.Errorf("apps[1] = %+v", resp.Apps[1])
	}
}

func TestHandleLogs_Defaults(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			return []byte("line1\nline2\n"), nil
		},
	}
	resp := handleLogs(r, &agent.LogsRequest{AppName: "myapp"})
	if !resp.OK {
		t.Fatalf("logs failed: %s", resp.Error)
	}
	if !r.called("--tail 100") {
		t.Errorf("lines<=0 must default to 100; calls: %v", r.calls)
	}
	if len(resp.Logs) != 2 || resp.Logs[0] != "line1" {
		t.Errorf("logs = %v", resp.Logs)
	}
}

// --- db create ---

func TestHandleDBCreate_RejectsInvalidIdentifiers(t *testing.T) {
	cases := []agent.DBCreateRequest{
		{User: "bad-user;", Database: "db", Password: "p"},
		{User: "u", Database: "db;DROP TABLE x", Password: "p"},
		{User: "u", Database: "db", Password: ""},
	}
	for _, req := range cases {
		r := &fakeRunner{}
		resp := handleDBCreate(r, &req)
		if resp.OK {
			t.Errorf("request %+v must be rejected", req)
		}
		if len(r.calls) != 0 {
			t.Errorf("invalid request must not execute anything; ran %v", r.calls)
		}
	}
}

func TestHandleDBCreate_Success(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker ps -q") {
				return []byte("abc123\n"), nil // postgres already running
			}
			return nil, nil
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "s3cret",
	})
	if !resp.OK {
		t.Fatalf("db create failed: %s", resp.Error)
	}
	if !r.called("CREATE USER myapp WITH PASSWORD 's3cret';") {
		t.Errorf("user was not created; calls: %v", r.calls)
	}
	if !r.called("CREATE DATABASE myapp OWNER myapp;") {
		t.Errorf("database was not created; calls: %v", r.calls)
	}
	if r.called("_ro") {
		t.Errorf("no ro_password given — no read-only role expected; calls: %v", r.calls)
	}
}

func TestHandleDBCreate_PasswordEscaped(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker ps -q") {
				return []byte("abc123\n"), nil
			}
			return nil, nil
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "it's",
	})
	if !resp.OK {
		t.Fatalf("db create failed: %s", resp.Error)
	}
	if !r.called("PASSWORD 'it''s'") {
		t.Errorf("single quote in password must be doubled; calls: %v", r.calls)
	}
}

func TestHandleDBCreate_AlreadyExists(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			switch {
			case strings.HasPrefix(cmd, "docker ps -q"):
				return []byte("abc123\n"), nil
			case strings.Contains(cmd, "CREATE DATABASE"):
				return []byte(`ERROR:  database "myapp" already exists`), exitErr{"exit status 1"}
			default:
				return nil, nil
			}
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "p",
	})
	if !resp.OK {
		t.Fatalf("already-exists must be treated as success: %s", resp.Error)
	}
	if !strings.Contains(resp.Message, "already exists") {
		t.Errorf("message = %q", resp.Message)
	}
}

func TestHandleDBCreate_ReadOnlyRole(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			if strings.HasPrefix(cmd, "docker ps -q") {
				return []byte("abc123\n"), nil
			}
			return nil, nil
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "p", ROPassword: "ro-pass",
	})
	if !resp.OK {
		t.Fatalf("db create with ro role failed: %s", resp.Error)
	}
	wantCalls := []string{
		"CREATE USER myapp_ro WITH PASSWORD 'ro-pass';",
		"GRANT CONNECT ON DATABASE myapp TO myapp_ro;",
		"GRANT USAGE ON SCHEMA public TO myapp_ro;",
		"GRANT SELECT ON ALL TABLES IN SCHEMA public TO myapp_ro;",
		"ALTER DEFAULT PRIVILEGES FOR ROLE myapp IN SCHEMA public GRANT SELECT ON TABLES TO myapp_ro;",
	}
	for _, want := range wantCalls {
		if !r.called(want) {
			t.Errorf("missing %q; calls: %v", want, r.calls)
		}
	}
	// Per-database grants must target the new database, not the
	// maintenance database.
	for _, c := range r.calls {
		if strings.Contains(c, "GRANT USAGE ON SCHEMA") && !strings.Contains(c, "-d myapp") {
			t.Errorf("schema grant must run against the app database: %q", c)
		}
	}
	if !strings.Contains(resp.Message, "myapp_ro") {
		t.Errorf("message should mention the ro role: %q", resp.Message)
	}
}

func TestHandleDBCreate_ReadOnlyRoleOnExistingDatabase(t *testing.T) {
	// Re-issuing db_create with ro_password against an existing database
	// must retrofit the role — that's how the fleet gets the security
	// fix without recreating databases.
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			switch {
			case strings.HasPrefix(cmd, "docker ps -q"):
				return []byte("abc123\n"), nil
			case strings.Contains(cmd, "CREATE DATABASE"):
				return []byte("already exists"), exitErr{"exit status 1"}
			default:
				return nil, nil
			}
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "p", ROPassword: "ro-pass",
	})
	if !resp.OK {
		t.Fatalf("retrofit failed: %s", resp.Error)
	}
	if !r.called("GRANT SELECT ON ALL TABLES IN SCHEMA public TO myapp_ro;") {
		t.Errorf("ro grants must run in the already-exists path too; calls: %v", r.calls)
	}
}

func TestHandleDBCreate_ReadOnlyGrantFailureIsFatal(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			switch {
			case strings.HasPrefix(cmd, "docker ps -q"):
				return []byte("abc123\n"), nil
			case strings.Contains(cmd, "GRANT CONNECT"):
				return []byte("permission denied"), exitErr{"exit status 1"}
			default:
				return nil, nil
			}
		},
	}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: "myapp", Password: "p", ROPassword: "ro-pass",
	})
	if resp.OK {
		t.Fatal("a failed grant leaves the ro role useless — must be reported as an error")
	}
	if !strings.Contains(resp.Error, "read-only role grant failed") {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestHandleDBCreate_ReadOnlyRoleNameTooLong(t *testing.T) {
	// 62 chars is a valid identifier, but +3 for "_ro" overflows
	// Postgres's 63-char limit.
	longUser := strings.Repeat("u", 62)
	r := &fakeRunner{}
	resp := handleDBCreate(r, &agent.DBCreateRequest{
		Database: "myapp", User: longUser, Password: "p", ROPassword: "ro",
	})
	if resp.OK {
		t.Fatal("overlong ro role name must be rejected")
	}
	if len(r.calls) != 0 {
		t.Errorf("invalid request must not execute anything; ran %v", r.calls)
	}
}

// --- db migrate ---

func TestHandleDBMigrate_QuotedArguments(t *testing.T) {
	r := &fakeRunner{}
	resp := handleDBMigrate(r, &agent.DBMigrateRequest{
		AppName:    "myapp",
		MigrateCmd: `bundle exec rails runner "User.where(active: true).count"`,
	})
	if !resp.OK {
		t.Fatalf("migrate failed: %s", resp.Error)
	}
	// The quoted ruby expression must arrive as ONE argument.
	want := "docker exec ezkeel-myapp bundle exec rails runner User.where(active: true).count"
	if len(r.calls) != 1 || r.calls[0] != want {
		t.Errorf("calls = %v, want [%q]", r.calls, want)
	}
}

func TestHandleDBMigrate_EmptyCommand(t *testing.T) {
	for _, cmd := range []string{"", "   "} {
		resp := handleDBMigrate(&fakeRunner{}, &agent.DBMigrateRequest{AppName: "a", MigrateCmd: cmd})
		if resp.OK || !strings.Contains(resp.Error, "empty") {
			t.Errorf("MigrateCmd %q: resp = %+v", cmd, resp)
		}
	}
}

func TestHandleDBMigrate_UnterminatedQuote(t *testing.T) {
	resp := handleDBMigrate(&fakeRunner{}, &agent.DBMigrateRequest{
		AppName: "a", MigrateCmd: `runner "oops`,
	})
	if resp.OK || !strings.Contains(resp.Error, "invalid migrate_cmd") {
		t.Errorf("resp = %+v", resp)
	}
}

// --- db backup ---

func TestHandleDBBackup_Success(t *testing.T) {
	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			return []byte("-- PostgreSQL dump\n"), nil
		},
	}
	resp := handleDBBackup(r, &agent.DBBackupRequest{Database: "myapp"})
	if !resp.OK {
		t.Fatalf("backup failed: %s", resp.Error)
	}
	if len(resp.Logs) != 1 || !strings.Contains(resp.Logs[0], "PostgreSQL dump") {
		t.Errorf("logs = %v", resp.Logs)
	}
}
