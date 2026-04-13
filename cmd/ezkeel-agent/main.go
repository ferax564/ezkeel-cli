package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/ferax564/ezkeel-cli/internal/version"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--request" {
		handleRequest()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("ezkeel-agent v%s\n", version.Version)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: ezkeel-agent --request | --version")
	os.Exit(1)
}

func handleRequest() {
	var req agent.Request
	dec := json.NewDecoder(os.Stdin)
	if err := dec.Decode(&req); err != nil {
		writeError("failed to decode request: " + err.Error())
		return
	}

	switch req.Type {
	case agent.CmdDeploy:
		if req.Deploy == nil {
			writeError("deploy request missing deploy payload")
			return
		}
		handleDeploy(req.Deploy)
	case agent.CmdStop:
		if req.Stop == nil {
			writeError("stop request missing stop payload")
			return
		}
		handleStop(req.Stop)
	case agent.CmdStatus:
		handleStatus()
	case agent.CmdLogs:
		if req.Logs == nil {
			writeError("logs request missing logs payload")
			return
		}
		handleLogs(req.Logs)
	case agent.CmdDBCreate:
		if req.DBCreate == nil {
			writeError("db_create request missing db_create payload")
			return
		}
		handleDBCreate(req.DBCreate)
	case agent.CmdDBMigrate:
		if req.DBMigrate == nil {
			writeError("db_migrate request missing db_migrate payload")
			return
		}
		handleDBMigrate(req.DBMigrate)
	case agent.CmdDBBackup:
		if req.DBBackup == nil {
			writeError("db_backup request missing payload")
			return
		}
		handleDBBackup(req.DBBackup)
	case agent.CmdRollback:
		if req.Rollback == nil {
			writeError("rollback request missing rollback payload")
			return
		}
		handleRollback(req.Rollback)
	default:
		writeError("unknown command type: " + string(req.Type))
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
func stopAndRemove(name string) {
	exec.Command("docker", "stop", name).Run() //nolint:errcheck
	exec.Command("docker", "rm", name).Run()   //nolint:errcheck
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

func handleDeploy(req *agent.DeployRequest) {
	name := containerName(req.AppName)
	prevTag := prevImageTag(req.AppName)

	inspectOut, err := exec.Command("docker", "inspect", "--format", "{{.Config.Image}}", name).Output()
	if err == nil {
		currentImage := strings.TrimSpace(string(inspectOut))
		if currentImage != "" {
			exec.Command("docker", "tag", currentImage, prevTag).Run() //nolint:errcheck
		}
	}

	stopAndRemove(name)

	args := buildRunArgs(name, req.Port, req.Memory, req.CPUs, req.Env, req.ImageTag)
	out, runErr := exec.Command("docker", args...).CombinedOutput()
	if runErr != nil {
		writeError(fmt.Sprintf("docker run failed: %s: %s", runErr.Error(), string(out)))
		return
	}

	writeOK(fmt.Sprintf("deployed %s (%s) on network ezkeel-apps", req.AppName, req.ImageTag))
}

func handleStop(req *agent.StopRequest) {
	name := containerName(req.AppName)

	out, err := exec.Command("docker", "stop", name).CombinedOutput()
	if err != nil {
		writeError(fmt.Sprintf("docker stop failed: %s: %s", err.Error(), string(out)))
		return
	}

	out, err = exec.Command("docker", "rm", name).CombinedOutput()
	if err != nil {
		writeError(fmt.Sprintf("docker rm failed: %s: %s", err.Error(), string(out)))
		return
	}

	writeOK("stopped " + req.AppName)
}

func handleStatus() {
	// List all ezkeel- containers (running and stopped).
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--filter", "name=ezkeel-",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}",
	).Output()
	if err != nil {
		writeError("docker ps failed: " + err.Error())
		return
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

	resp := agent.Response{
		OK:   true,
		Apps: apps,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(resp) //nolint:errcheck
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

func handleLogs(req *agent.LogsRequest) {
	name := containerName(req.AppName)
	lines := req.Lines
	if lines <= 0 {
		lines = 100
	}

	out, err := exec.Command(
		"docker", "logs",
		"--tail", strconv.Itoa(lines),
		name,
	).CombinedOutput()
	if err != nil {
		writeError(fmt.Sprintf("docker logs failed: %s: %s", err.Error(), string(out)))
		return
	}

	var logLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}

	resp := agent.Response{
		OK:   true,
		Logs: logLines,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(resp) //nolint:errcheck
}

func handleDBCreate(req *agent.DBCreateRequest) {
	// Validate identifiers to prevent SQL injection.
	if !validIdentifier.MatchString(req.User) {
		writeError(fmt.Sprintf("invalid database user name: %q", req.User))
		return
	}
	if !validIdentifier.MatchString(req.Database) {
		writeError(fmt.Sprintf("invalid database name: %q", req.Database))
		return
	}
	if req.Password == "" {
		writeError("database password must not be empty")
		return
	}

	// Start the postgres container if it is not already running.
	checkOut, _ := exec.Command(
		"docker", "ps", "-q", "--filter", "name="+pgContainerName(),
	).Output()

	if strings.TrimSpace(string(checkOut)) == "" {
		// Container is not running — start or create it.
		version := req.Version
		if version == "" {
			version = "16"
		}
		if !validIdentifier.MatchString(version) {
			writeError(fmt.Sprintf("invalid postgres version: %q", version))
			return
		}
		image := fmt.Sprintf("postgres:%s-alpine", version)

		startArgs := []string{
			"run", "-d",
			"--name", pgContainerName(),
			"--restart", "unless-stopped",
		}
		startArgs = append(startArgs, image)

		out, err := exec.Command("docker", startArgs...).CombinedOutput()
		if err != nil {
			// Container may already exist but be stopped — try starting it.
			out2, err2 := exec.Command("docker", "start", pgContainerName()).CombinedOutput()
			if err2 != nil {
				writeError(fmt.Sprintf(
					"could not start postgres container: run: %s (%s), start: %s (%s)",
					err.Error(), string(out), err2.Error(), string(out2),
				))
				return
			}
		}
	}

	// Create user (ignore error if already exists).
	// Identifier is validated above; password is escaped inside single quotes.
	createUserSQL := fmt.Sprintf(
		"CREATE USER %s WITH PASSWORD '%s';",
		req.User, escapeSQLString(req.Password),
	)
	exec.Command( //nolint:errcheck
		"docker", "exec", pgContainerName(),
		"psql", "-U", "ezkeel", "-c", createUserSQL,
	).Run()

	// Create database owned by user (both identifiers validated above).
	createDBSQL := fmt.Sprintf(
		"CREATE DATABASE %s OWNER %s;",
		req.Database, req.User,
	)
	out, err := exec.Command(
		"docker", "exec", pgContainerName(),
		"psql", "-U", "ezkeel", "-c", createDBSQL,
	).CombinedOutput()
	if err != nil {
		// Database may already exist — treat as success if output says so.
		if strings.Contains(string(out), "already exists") {
			writeOK(fmt.Sprintf("database %s already exists", req.Database))
			return
		}
		writeError(fmt.Sprintf("create database failed: %s: %s", err.Error(), string(out)))
		return
	}

	writeOK(fmt.Sprintf("created database %s with user %s", req.Database, req.User))
}

func handleDBMigrate(req *agent.DBMigrateRequest) {
	name := containerName(req.AppName)

	// Split the migrate command into tokens for exec.
	parts := strings.Fields(req.MigrateCmd)
	if len(parts) == 0 {
		writeError("migrate_cmd is empty")
		return
	}

	args := append([]string{"exec", name}, parts...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		writeError(fmt.Sprintf("migration failed: %s: %s", err.Error(), string(out)))
		return
	}

	writeOK("migration completed: " + strings.TrimSpace(string(out)))
}

func handleDBBackup(req *agent.DBBackupRequest) {
	cmd := exec.Command("docker", "exec", pgContainerName(),
		"pg_dump", "-U", "ezkeel", req.Database)
	out, err := cmd.Output()
	if err != nil {
		writeError(fmt.Sprintf("pg_dump failed: %v", err))
		return
	}
	msg := fmt.Sprintf("backup of %s (%d bytes)", req.Database, len(out))
	resp := agent.Response{OK: true, Message: msg, Logs: []string{string(out)}}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(resp) //nolint:errcheck
}

func handleRollback(req *agent.RollbackRequest) {
	name := containerName(req.AppName)
	prevTag := prevImageTag(req.AppName)

	if err := exec.Command("docker", "image", "inspect", prevTag).Run(); err != nil {
		writeError(fmt.Sprintf("no previous image found for %s — nothing to roll back to", req.AppName))
		return
	}

	stopAndRemove(name)

	args := buildRunArgs(name, req.Port, req.Memory, req.CPUs, nil, prevTag)
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		writeError(fmt.Sprintf("rollback failed: %s: %s", err.Error(), string(out)))
		return
	}

	writeOK(fmt.Sprintf("rolled back %s to previous version", req.AppName))
}

func writeOK(msg string) {
	resp := agent.Response{OK: true, Message: msg}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(resp) //nolint:errcheck
}

func writeError(msg string) {
	resp := agent.Response{OK: false, Error: msg}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(resp) //nolint:errcheck
}
