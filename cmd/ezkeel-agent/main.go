package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/version"
	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

// validIdentifier matches safe PostgreSQL identifiers: starts with letter or
// underscore, followed by alphanumeric or underscore, max 63 chars.
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// escapeSQLString escapes a string for use inside single-quoted SQL literals
// by doubling any single-quote characters.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// pgContainerName returns the Postgres container name on the target VPS.
// Defaults to "ezkeel-postgres-1" to match the docker-compose naming
// convention (<project>_<service>_<instance>). Override with
// EZKEEL_PG_CONTAINER if your deployment uses a different name.
//
// Historical note: this used to be a bare "ezkeel-postgres" constant,
// which never matched the actually-running container and caused every
// db_create request to fail with "No such container".
func pgContainerName() string {
	if v := os.Getenv("EZKEEL_PG_CONTAINER"); v != "" {
		return v
	}
	return "ezkeel-postgres-1"
}

// runner abstracts process execution so handlers can be tested without
// docker (or root) on the host. The method shapes mirror the exec.Cmd
// calls the handlers were written against.
type runner interface {
	// Output returns the command's stdout only.
	Output(name string, args ...string) ([]byte, error)
	// CombinedOutput returns interleaved stdout+stderr.
	CombinedOutput(name string, args ...string) ([]byte, error)
	// Run discards output and reports only the exit status.
	Run(name string, args ...string) error
}

// systemRunner is the production runner backed by os/exec.
type systemRunner struct{}

func (systemRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (systemRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (systemRunner) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--request" {
		handleRequest(os.Stdin, os.Stdout)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("ezkeel-agent v%s\n", version.Version)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: ezkeel-agent --request | --version")
	os.Exit(1)
}

// handleRequest decodes one request from in, dispatches it, stamps the
// response with this agent's protocol and build versions, and encodes
// it to out.
func handleRequest(in io.Reader, out io.Writer) {
	var resp *agent.Response

	var req agent.Request
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		resp = respErr("failed to decode request: " + err.Error())
	} else {
		resp = dispatch(systemRunner{}, &req)
	}

	resp.ProtocolVersion = agent.CurrentProtocolVersion
	resp.AgentVersion = version.Version
	json.NewEncoder(out).Encode(resp) //nolint:errcheck
}

// dispatch routes a decoded request to its handler and returns the
// response to send.
func dispatch(r runner, req *agent.Request) *agent.Response {
	// Reject requests from a NEWER protocol than this build understands:
	// they may carry command types or fields we'd silently mishandle.
	// Older (including pre-versioning, version 0) requests are fine —
	// the protocol only grows additively.
	if req.ProtocolVersion > agent.CurrentProtocolVersion {
		return respErr(fmt.Sprintf(
			"request speaks protocol version %d but this agent (v%s) only speaks %d — update the agent first",
			req.ProtocolVersion, version.Version, agent.CurrentProtocolVersion,
		))
	}

	switch req.Type {
	case agent.CmdDeploy:
		if req.Deploy == nil {
			return respErr("deploy request missing deploy payload")
		}
		return handleDeploy(r, req.Deploy)
	case agent.CmdStop:
		if req.Stop == nil {
			return respErr("stop request missing stop payload")
		}
		return handleStop(r, req.Stop)
	case agent.CmdStatus:
		return handleStatus(r)
	case agent.CmdLogs:
		if req.Logs == nil {
			return respErr("logs request missing logs payload")
		}
		return handleLogs(r, req.Logs)
	case agent.CmdDBCreate:
		if req.DBCreate == nil {
			return respErr("db_create request missing db_create payload")
		}
		return handleDBCreate(r, req.DBCreate)
	case agent.CmdDBMigrate:
		if req.DBMigrate == nil {
			return respErr("db_migrate request missing db_migrate payload")
		}
		return handleDBMigrate(r, req.DBMigrate)
	case agent.CmdDBBackup:
		if req.DBBackup == nil {
			return respErr("db_backup request missing payload")
		}
		return handleDBBackup(r, req.DBBackup)
	case agent.CmdRollback:
		if req.Rollback == nil {
			return respErr("rollback request missing rollback payload")
		}
		return handleRollback(r, req.Rollback)
	case agent.CmdVersion:
		return handleVersion()
	case agent.CmdUpdate:
		// A nil payload is a valid "update to latest" request.
		upd := req.Update
		if upd == nil {
			upd = &agent.UpdateRequest{}
		}
		return handleUpdate(r, upd, defaultUpdateOptions())
	default:
		return respErr("unknown command type: " + string(req.Type))
	}
}

// sanitizeName returns a safe container-name component (alphanumeric + dash).
func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containerName(appName string) string {
	return "ezkeel-" + sanitizeName(appName)
}

func prevImageTag(appName string) string {
	return sanitizeName(appName) + ":prev"
}

// stopAndRemove stops and removes a container, ignoring errors (container may not exist).
func stopAndRemove(r runner, name string) {
	r.Run("docker", "stop", name) //nolint:errcheck
	r.Run("docker", "rm", name)   //nolint:errcheck
}

// buildRunArgs constructs the docker run argument list for an app container.
func buildRunArgs(name string, port int, memory, cpus string, env map[string]string, image string) []string {
	args := []string{
		"run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", "ezkeel-apps",
	}
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if memory != "" {
		args = append(args, "--memory", memory)
	}
	if cpus != "" {
		args = append(args, "--cpus", cpus)
	}
	args = append(args, image)
	return args
}

func handleDeploy(r runner, req *agent.DeployRequest) *agent.Response {
	name := containerName(req.AppName)
	prevTag := prevImageTag(req.AppName)

	// Tag the currently-running image as :prev so rollback has a target.
	// A failed tag is NOT fatal to the deploy, but it must be surfaced:
	// auto-rollback depends on :prev existing, so the operator needs to
	// know it won't be available for this app.
	tagWarning := ""
	inspectOut, err := r.Output("docker", "inspect", "--format", "{{.Config.Image}}", name)
	if err == nil {
		currentImage := strings.TrimSpace(string(inspectOut))
		if currentImage != "" {
			if tagOut, tagErr := r.CombinedOutput("docker", "tag", currentImage, prevTag); tagErr != nil {
				tagWarning = fmt.Sprintf(
					"; warning: could not tag %s as %s — rollback will not be available for this deploy: %s: %s",
					currentImage, prevTag, tagErr.Error(), strings.TrimSpace(string(tagOut)),
				)
			}
		}
	}

	stopAndRemove(r, name)

	args := buildRunArgs(name, req.Port, req.Memory, req.CPUs, req.Env, req.ImageTag)
	out, runErr := r.CombinedOutput("docker", args...)
	if runErr != nil {
		return respErr(fmt.Sprintf("docker run failed: %s: %s", runErr.Error(), string(out)))
	}

	return respOK(fmt.Sprintf("deployed %s (%s) on network ezkeel-apps%s", req.AppName, req.ImageTag, tagWarning))
}

func handleStop(r runner, req *agent.StopRequest) *agent.Response {
	name := containerName(req.AppName)

	out, err := r.CombinedOutput("docker", "stop", name)
	if err != nil {
		return respErr(fmt.Sprintf("docker stop failed: %s: %s", err.Error(), string(out)))
	}

	out, err = r.CombinedOutput("docker", "rm", name)
	if err != nil {
		return respErr(fmt.Sprintf("docker rm failed: %s: %s", err.Error(), string(out)))
	}

	return respOK("stopped " + req.AppName)
}

func handleStatus(r runner) *agent.Response {
	// List all ezkeel- containers (running and stopped).
	out, err := r.Output(
		"docker", "ps", "-a",
		"--filter", "name=ezkeel-",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}",
	)
	if err != nil {
		return respErr("docker ps failed: " + err.Error())
	}

	var apps []agent.AppStatus
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		rawName := parts[0]
		status := parts[1]
		portsField := ""
		if len(parts) == 3 {
			portsField = parts[2]
		}

		// Derive app name: strip "ezkeel-" prefix.
		appName := strings.TrimPrefix(rawName, "ezkeel-")

		// Parse port from ports field, e.g. "127.0.0.1:3000->3000/tcp".
		port := parsePort(portsField)

		// Determine canonical status.
		canonicalStatus := "stopped"
		lower := strings.ToLower(status)
		if strings.HasPrefix(lower, "up") {
			canonicalStatus = "running"
		} else if strings.Contains(lower, "exited") {
			canonicalStatus = "stopped"
		}

		url := ""
		if port > 0 {
			url = fmt.Sprintf("http://localhost:%d", port)
		}

		apps = append(apps, agent.AppStatus{
			Name:   appName,
			Status: canonicalStatus,
			Port:   port,
			URL:    url,
		})
	}

	return &agent.Response{OK: true, Apps: apps}
}

// parsePort extracts the host port from a Docker ports string like
// "127.0.0.1:3000->3000/tcp" or "0.0.0.0:3000->3000/tcp".
func parsePort(ports string) int {
	if ports == "" {
		return 0
	}
	// Take the first mapping.
	entry := strings.SplitN(ports, ",", 2)[0]
	entry = strings.TrimSpace(entry)
	// entry looks like "127.0.0.1:3000->3000/tcp"
	hostPart := strings.SplitN(entry, "->", 2)[0]
	// hostPart is "127.0.0.1:3000" or "3000"
	colonIdx := strings.LastIndex(hostPart, ":")
	if colonIdx == -1 {
		p, _ := strconv.Atoi(hostPart)
		return p
	}
	p, _ := strconv.Atoi(hostPart[colonIdx+1:])
	return p
}

func handleLogs(r runner, req *agent.LogsRequest) *agent.Response {
	name := containerName(req.AppName)
	lines := req.Lines
	if lines <= 0 {
		lines = 100
	}

	out, err := r.CombinedOutput(
		"docker", "logs",
		"--tail", strconv.Itoa(lines),
		name,
	)
	if err != nil {
		return respErr(fmt.Sprintf("docker logs failed: %s: %s", err.Error(), string(out)))
	}

	var logLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}

	return &agent.Response{OK: true, Logs: logLines}
}

func handleDBCreate(r runner, req *agent.DBCreateRequest) *agent.Response {
	// Validate identifiers to prevent SQL injection.
	if !validIdentifier.MatchString(req.User) {
		return respErr(fmt.Sprintf("invalid database user name: %q", req.User))
	}
	if !validIdentifier.MatchString(req.Database) {
		return respErr(fmt.Sprintf("invalid database name: %q", req.Database))
	}
	if req.Password == "" {
		return respErr("database password must not be empty")
	}
	roUser := req.User + "_ro"
	if req.ROPassword != "" && !validIdentifier.MatchString(roUser) {
		// The _ro suffix can push an otherwise-valid 62/63-char user
		// name past Postgres's identifier limit.
		return respErr(fmt.Sprintf("invalid read-only role name: %q", roUser))
	}

	// Start the postgres container if it is not already running.
	checkOut, _ := r.Output(
		"docker", "ps", "-q", "--filter", "name="+pgContainerName(),
	)

	if strings.TrimSpace(string(checkOut)) == "" {
		// Container is not running — start or create it.
		pgVersion := req.Version
		if pgVersion == "" {
			pgVersion = "16"
		}
		if !validIdentifier.MatchString(pgVersion) {
			return respErr(fmt.Sprintf("invalid postgres version: %q", pgVersion))
		}
		image := fmt.Sprintf("postgres:%s-alpine", pgVersion)

		startArgs := []string{
			"run", "-d",
			"--name", pgContainerName(),
			"--restart", "unless-stopped",
		}
		startArgs = append(startArgs, image)

		out, err := r.CombinedOutput("docker", startArgs...)
		if err != nil {
			// Container may already exist but be stopped — try starting it.
			out2, err2 := r.CombinedOutput("docker", "start", pgContainerName())
			if err2 != nil {
				return respErr(fmt.Sprintf(
					"could not start postgres container: run: %s (%s), start: %s (%s)",
					err.Error(), string(out), err2.Error(), string(out2),
				))
			}
		}
	}

	// Create user (ignore error if already exists).
	// Identifier is validated above; password is escaped inside single quotes.
	createUserSQL := fmt.Sprintf(
		"CREATE USER %s WITH PASSWORD '%s';",
		req.User, escapeSQLString(req.Password),
	)
	r.Run( //nolint:errcheck
		"docker", "exec", pgContainerName(),
		"psql", "-U", "ezkeel", "-c", createUserSQL,
	)

	// Create database owned by user (both identifiers validated above).
	dbExisted := false
	createDBSQL := fmt.Sprintf(
		"CREATE DATABASE %s OWNER %s;",
		req.Database, req.User,
	)
	out, err := r.CombinedOutput(
		"docker", "exec", pgContainerName(),
		"psql", "-U", "ezkeel", "-c", createDBSQL,
	)
	if err != nil {
		// Database may already exist — treat as success if output says so.
		if !strings.Contains(string(out), "already exists") {
			return respErr(fmt.Sprintf("create database failed: %s: %s", err.Error(), string(out)))
		}
		dbExisted = true
	}

	// Optionally provision the read-only companion role. Runs in the
	// already-exists path too, so re-issuing db_create with ro_password
	// retrofits the role onto an existing database.
	if req.ROPassword != "" {
		if resp := createReadOnlyRole(r, req, roUser); resp != nil {
			return resp
		}
	}

	if dbExisted {
		msg := fmt.Sprintf("database %s already exists", req.Database)
		if req.ROPassword != "" {
			msg += fmt.Sprintf("; read-only role %s ensured", roUser)
		}
		return respOK(msg)
	}
	msg := fmt.Sprintf("created database %s with user %s", req.Database, req.User)
	if req.ROPassword != "" {
		msg += fmt.Sprintf(" and read-only role %s", roUser)
	}
	return respOK(msg)
}

// createReadOnlyRole provisions roUser as a SELECT-only login role on
// req.Database: CONNECT + USAGE on schema public + SELECT on current
// tables, plus a default privilege so tables req.User creates later are
// covered. Idempotent — CREATE USER failure is ignored like the
// read-write branch, and GRANT/ALTER DEFAULT PRIVILEGES re-apply
// cleanly. Returns nil on success, an error response otherwise (the
// grants are the security boundary, so unlike the role creation their
// failures are fatal).
func createReadOnlyRole(r runner, req *agent.DBCreateRequest, roUser string) *agent.Response {
	createRoSQL := fmt.Sprintf(
		"CREATE USER %s WITH PASSWORD '%s';",
		roUser, escapeSQLString(req.ROPassword),
	)
	r.Run( //nolint:errcheck
		"docker", "exec", pgContainerName(),
		"psql", "-U", "ezkeel", "-c", createRoSQL,
	)

	grants := []struct {
		db  string // "" = default maintenance database
		sql string
	}{
		{"", fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;", req.Database, roUser)},
		{req.Database, fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s;", roUser)},
		{req.Database, fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s;", roUser)},
		{req.Database, fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA public GRANT SELECT ON TABLES TO %s;",
			req.User, roUser,
		)},
	}
	for _, g := range grants {
		args := []string{"exec", pgContainerName(), "psql", "-U", "ezkeel"}
		if g.db != "" {
			args = append(args, "-d", g.db)
		}
		args = append(args, "-c", g.sql)
		if out, err := r.CombinedOutput("docker", args...); err != nil {
			return respErr(fmt.Sprintf("read-only role grant failed (%s): %s: %s", g.sql, err.Error(), string(out)))
		}
	}
	return nil
}

func handleDBMigrate(r runner, req *agent.DBMigrateRequest) *agent.Response {
	name := containerName(req.AppName)

	// Split the migrate command into tokens for exec, honouring shell
	// quoting so commands like `rails runner "puts 'x'"` survive intact.
	parts, err := splitShellWords(req.MigrateCmd)
	if err != nil {
		return respErr("invalid migrate_cmd: " + err.Error())
	}
	if len(parts) == 0 {
		return respErr("migrate_cmd is empty")
	}

	args := append([]string{"exec", name}, parts...)
	out, err := r.CombinedOutput("docker", args...)
	if err != nil {
		return respErr(fmt.Sprintf("migration failed: %s: %s", err.Error(), string(out)))
	}

	return respOK("migration completed: " + strings.TrimSpace(string(out)))
}

func handleDBBackup(r runner, req *agent.DBBackupRequest) *agent.Response {
	out, err := r.Output("docker", "exec", pgContainerName(),
		"pg_dump", "-U", "ezkeel", req.Database)
	if err != nil {
		return respErr(fmt.Sprintf("pg_dump failed: %v", err))
	}
	msg := fmt.Sprintf("backup of %s (%d bytes)", req.Database, len(out))
	return &agent.Response{OK: true, Message: msg, Logs: []string{string(out)}}
}

func handleRollback(r runner, req *agent.RollbackRequest) *agent.Response {
	name := containerName(req.AppName)
	prevTag := prevImageTag(req.AppName)

	if err := r.Run("docker", "image", "inspect", prevTag); err != nil {
		return respErr(fmt.Sprintf("no previous image found for %s — nothing to roll back to", req.AppName))
	}

	stopAndRemove(r, name)

	// req.Env carries the deploy-time environment (DATABASE_URL + user
	// env). Without it the fresh :prev container starts bare and a
	// DB-backed app crash-loops — the pre-fix behaviour.
	args := buildRunArgs(name, req.Port, req.Memory, req.CPUs, req.Env, prevTag)
	out, err := r.CombinedOutput("docker", args...)
	if err != nil {
		return respErr(fmt.Sprintf("rollback failed: %s: %s", err.Error(), string(out)))
	}

	return respOK(fmt.Sprintf("rolled back %s to previous version", req.AppName))
}

// handleVersion answers the side-effect-free version probe. The
// interesting data (agent_version, protocol_version) is stamped on
// every response by handleRequest; the message is for humans.
func handleVersion() *agent.Response {
	return respOK(fmt.Sprintf("ezkeel-agent v%s (protocol %d)", version.Version, agent.CurrentProtocolVersion))
}

func respOK(msg string) *agent.Response {
	return &agent.Response{OK: true, Message: msg}
}

func respErr(msg string) *agent.Response {
	return &agent.Response{OK: false, Error: msg}
}
