package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/ferax564/ezkeel-cli/internal/version"
	"github.com/spf13/cobra"
)

type checkResult struct {
	Name   string
	OK     bool
	Detail string
}

// parseDiskUsagePct extracts the Use% value from a single line of df output.
func parseDiskUsagePct(line string) (int, error) {
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.HasSuffix(f, "%") {
			num := strings.TrimSuffix(f, "%")
			return strconv.Atoi(num)
		}
	}
	return 0, fmt.Errorf("no percentage found in %q", line)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health and server connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")

		var srv *config.Server
		var err error
		if serverName != "" {
			srv, err = config.LoadServer(serverName)
		} else {
			srv, err = config.DefaultServer()
		}

		fmt.Printf("%s doctor v%s\n\n", tui.Banner(), version.Version)

		// Check 1: Local Docker
		var results []checkResult
		dockerOut, dockerErr := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
		if dockerErr != nil {
			results = append(results, checkResult{"Local Docker", false, "not found — install Docker"})
		} else {
			results = append(results, checkResult{"Local Docker", true, "v" + strings.TrimSpace(string(dockerOut))})
		}

		// If no server configured, show local-only results
		if err != nil {
			results = append(results, checkResult{"Server", false, "no server configured — run 'ezkeel server add'"})
			printResults(results)
			return nil
		}

		client := clientFromServer(srv)
		ctx := cmd.Context()

		// Check 2: SSH connectivity
		sshOut, sshErr := client.RunRemote(ctx, "echo ok")
		if sshErr != nil {
			results = append(results, checkResult{"SSH to " + srv.Name, false, "connection failed: " + sshErr.Error()})
			printResults(results)
			return nil
		}
		if strings.TrimSpace(sshOut) == "ok" {
			results = append(results, checkResult{"SSH to " + srv.Name, true, srv.Host})
		} else {
			results = append(results, checkResult{"SSH to " + srv.Name, false, "unexpected response"})
		}

		// Check 3: Remote Docker
		remoteDocker, dockerRemoteErr := client.RunRemote(ctx, "docker version --format '{{.Server.Version}}'")
		if dockerRemoteErr != nil {
			results = append(results, checkResult{"Remote Docker", false, "not installed on " + srv.Name})
		} else {
			results = append(results, checkResult{"Remote Docker", true, "v" + strings.TrimSpace(remoteDocker)})
		}

		// Check 4: Agent version
		agentVer, agentErr := client.RunRemote(ctx, "ezkeel-agent --version")
		if agentErr != nil {
			results = append(results, checkResult{"Agent", false, "not installed on " + srv.Name})
		} else {
			ver := strings.TrimSpace(agentVer)
			mismatch := !strings.Contains(ver, version.Version)
			if mismatch {
				results = append(results, checkResult{"Agent", false, ver + " (CLI is v" + version.Version + " — version mismatch)"})
			} else {
				results = append(results, checkResult{"Agent", true, ver})
			}
		}

		// Check 5: DNS wildcard
		if srv.Domain != "" {
			dnsOut, dnsErr := client.RunRemote(ctx, fmt.Sprintf("dig +short test-ezkeel-doctor.%s A 2>/dev/null || echo fail", srv.Domain))
			dns := strings.TrimSpace(dnsOut)
			if dnsErr != nil || dns == "" || dns == "fail" {
				results = append(results, checkResult{"DNS (*." + srv.Domain + ")", false, "wildcard not resolving — check DNS A record"})
			} else {
				results = append(results, checkResult{"DNS (*." + srv.Domain + ")", true, "resolves to " + dns})
			}
		}

		// Check 6: Disk space
		dfOut, dfErr := client.RunRemote(ctx, "df -h / | tail -1")
		if dfErr != nil {
			results = append(results, checkResult{"Disk Space", false, "could not check"})
		} else {
			pct, parseErr := parseDiskUsagePct(dfOut)
			if parseErr != nil {
				results = append(results, checkResult{"Disk Space", false, "could not parse: " + dfOut})
			} else if pct >= 90 {
				results = append(results, checkResult{"Disk Space", false, fmt.Sprintf("%d%% used — critically low", pct)})
			} else {
				results = append(results, checkResult{"Disk Space", true, fmt.Sprintf("%d%% used", pct)})
			}
		}

		// Check 7: Container security — exposed DB ports + trust auth
		results = append(results, runContainerSecurityChecks(ctx, client)...)

		printResults(results)
		return nil
	},
}

// dangerousDBPorts maps host-published ports that must never be public.
// Postgres (5432), MySQL (3306), MongoDB (27017), Redis (6379), Memcached (11211).
var dangerousDBPorts = []string{"5432", "3306", "27017", "6379", "11211"}

// runContainerSecurityChecks inspects running containers for two recurring
// operator mistakes that have led to compromise: DB ports bound to 0.0.0.0
// and trust-mode Postgres auth. Returns one checkResult per finding plus
// an "OK" result if nothing dangerous was found.
func runContainerSecurityChecks(ctx remoteCtx, client remoteClient) []checkResult {
	var out []checkResult

	psOut, psErr := client.RunRemote(ctx, `docker ps --format '{{.Names}}|{{.Ports}}'`)
	if psErr != nil {
		return []checkResult{{"Container Ports", false, "could not list containers: " + psErr.Error()}}
	}

	var exposed []string
	for _, line := range strings.Split(strings.TrimSpace(psOut), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		name, ports := parts[0], parts[1]
		for _, port := range dangerousDBPorts {
			// "0.0.0.0:5432->5432/tcp" or ":::5432->5432/tcp" = exposed publicly
			if strings.Contains(ports, "0.0.0.0:"+port+"->") || strings.Contains(ports, ":::"+port+"->") {
				exposed = append(exposed, fmt.Sprintf("%s :%s", name, port))
			}
		}
	}

	if len(exposed) > 0 {
		out = append(out, checkResult{"Container Ports", false, "DB port published to 0.0.0.0 — " + strings.Join(exposed, ", ") + " (rebind to 127.0.0.1 or remove)"})
	} else {
		out = append(out, checkResult{"Container Ports", true, "no DB ports bound to 0.0.0.0"})
	}

	// trust-mode Postgres auth = anyone who can connect becomes superuser.
	// Inspect every container's env for the smoking gun.
	envOut, envErr := client.RunRemote(ctx, `for c in $(docker ps --format '{{.Names}}'); do echo "$c|$(docker inspect "$c" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep -E '^POSTGRES_HOST_AUTH_METHOD=' || true)"; done`)
	if envErr == nil {
		var trustContainers []string
		for _, line := range strings.Split(strings.TrimSpace(envOut), "\n") {
			parts := strings.SplitN(line, "|", 2)
			if len(parts) == 2 && strings.Contains(parts[1], "POSTGRES_HOST_AUTH_METHOD=trust") {
				trustContainers = append(trustContainers, parts[0])
			}
		}
		if len(trustContainers) > 0 {
			out = append(out, checkResult{"Postgres Auth", false, "trust auth enabled — " + strings.Join(trustContainers, ", ") + " (superuser without password)"})
		} else {
			out = append(out, checkResult{"Postgres Auth", true, "no trust-mode containers"})
		}
	}

	return out
}

// remoteCtx and remoteClient match the existing client signature without
// importing the concrete types here, keeping this file self-contained.
type remoteCtx = context.Context
type remoteClient interface {
	RunRemote(ctx context.Context, cmd string) (string, error)
}

func printResults(results []checkResult) {
	for _, r := range results {
		if r.OK {
			fmt.Printf("  %s %s  %s\n", tui.IconDone, r.Name, tui.DimStyle.Render(r.Detail))
		} else {
			fmt.Printf("  %s %s  %s\n", tui.IconFail, r.Name, tui.ErrorStyle.Render(r.Detail))
		}
	}
	fmt.Println()
}

func init() {
	doctorCmd.Flags().String("server", "", "Server to check (default: first configured)")
}
