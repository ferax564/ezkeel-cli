package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/pkg/bootstrap"
	hetznerPkg "github.com/ferax564/ezkeel-cli/pkg/hetzner"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage deployment servers",
}

var serverAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Bootstrap a VPS with Docker, Caddy, and the ezkeel agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		useHetzner, _ := cmd.Flags().GetBool("hetzner")
		host, _ := cmd.Flags().GetString("host")
		domain, _ := cmd.Flags().GetString("domain")
		name, _ := cmd.Flags().GetString("name")
		user, _ := cmd.Flags().GetString("user")
		key, _ := cmd.Flags().GetString("key")
		sshAlias, _ := cmd.Flags().GetString("ssh-alias")
		bootstrapFlag, _ := cmd.Flags().GetBool(bootstrapFlagName)

		if domain == "" {
			return fmt.Errorf("--domain is required")
		}

		if useHetzner && cmd.Flags().Changed(bootstrapFlagName) && !bootstrapFlag {
			return fmt.Errorf("--hetzner provisions a fresh VPS that requires bootstrap; remove --bootstrap=false")
		}

		if useHetzner {
			token, _ := cmd.Flags().GetString("hetzner-token")
			if token == "" {
				token = os.Getenv("HETZNER_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("--hetzner-token or HETZNER_TOKEN env is required")
			}
			serverType, _ := cmd.Flags().GetString("hetzner-type")
			location, _ := cmd.Flags().GetString("hetzner-location")
			sshKeyName, _ := cmd.Flags().GetString("hetzner-ssh-key")
			if sshKeyName == "" {
				return fmt.Errorf("--hetzner-ssh-key is required (name of SSH key in Hetzner Cloud console)")
			}

			if name == "" {
				name = "ezkeel-vps"
			}

			fmt.Printf("Provisioning Hetzner VPS (%s in %s)...\n", serverType, location)
			hc := hetznerPkg.NewClient(token)
			result, err := hc.CreateServer(name, serverType, location, sshKeyName)
			if err != nil {
				return fmt.Errorf("hetzner create server: %w", err)
			}

			host = result.Server.PublicNet.IPv4.IP
			fmt.Printf("Server created: %s (ID: %d)\n", host, result.Server.ID)

			// Wait for server to be running.
			fmt.Print("Waiting for server to be ready...")
			serverReady := false
			for i := 0; i < 30; i++ {
				status, getErr := hc.GetServer(result.Server.ID)
				if getErr == nil && status.Server.Status == "running" {
					fmt.Println(" ready!")
					serverReady = true
					break
				}
				time.Sleep(2 * time.Second)
				fmt.Print(".")
			}
			if !serverReady {
				fmt.Println(" warning: server may not be ready yet, check Hetzner console")
			}
		}

		if host == "" && sshAlias == "" {
			return fmt.Errorf("--host or --ssh-alias is required")
		}
		if host == "" {
			host = sshAlias
		}

		name = serverNameFromHost(host, name)

		srv := &config.Server{
			Name:     name,
			Host:     host,
			User:     user,
			SSHKey:   key,
			SSHAlias: sshAlias,
			Domain:   domain,
		}

		if err := config.SaveServer(srv); err != nil {
			return fmt.Errorf("saving server: %w", err)
		}

		fmt.Printf("Server %q saved.\n", name)

		// bootstrap default is true (set in init); we still honour
		// an explicit --bootstrap=false from the operator.
		shouldBootstrap := bootstrapFlag

		if shouldBootstrap {
			ctx := cmd.Context()
			fmt.Println("Bootstrapping server (this can take ~60s on a fresh box)...")

			runner := runnerForServer(srv)
			if err := runBootstrap(ctx, runner, bootstrap.Options{}); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}

			client := clientFromServer(srv)

			fmt.Print("  Ensuring ezkeel-apps network... ")
			if _, err := client.RunRemote(ctx, "docker network inspect ezkeel-apps >/dev/null 2>&1"); err != nil {
				if _, cerr := client.RunRemote(ctx, "docker network create ezkeel-apps"); cerr != nil {
					return fmt.Errorf("creating ezkeel-apps network: %w", cerr)
				}
			}
			fmt.Println("done")

			fmt.Print("  Ensuring Caddy on ezkeel-apps... ")
			// `docker network connect` errors with non-zero when the
			// container is already attached. Probe membership first
			// via `docker network inspect -f` listing connected
			// containers; if ezkeel-caddy-1 is in the list, skip.
			if _, err := client.RunRemote(ctx, "docker network inspect ezkeel-apps -f '{{range .Containers}}{{.Name}}\n{{end}}' | grep -q '^ezkeel-caddy-1$'"); err != nil {
				if _, cerr := client.RunRemote(ctx, "docker network connect ezkeel-apps ezkeel-caddy-1"); cerr != nil {
					return fmt.Errorf("connecting caddy to ezkeel-apps: %w", cerr)
				}
			}
			fmt.Println("done")

			fmt.Printf("\nServer %q ready for deployments.\n", name)
			fmt.Printf("Deploy with: ezkeel up github.com/user/repo\n")
		} else {
			fmt.Printf("\nServer %q saved. Bootstrap was skipped (--bootstrap=false). The box must already have Docker, the ezkeel-agent binary, and Caddy connected to the ezkeel-apps network.\n", name)
		}

		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := config.ListServers()
		if err != nil {
			return fmt.Errorf("listing servers: %w", err)
		}
		if len(servers) == 0 {
			fmt.Println("No servers configured. Use 'ezkeel server add' to add one.")
			return nil
		}
		fmt.Printf("%-20s %-30s %-10s %s\n", "NAME", "HOST", "USER", "DOMAIN")
		fmt.Println(strings.Repeat("-", 75))
		for _, srv := range servers {
			user := srv.User
			if user == "" {
				user = "root"
			}
			fmt.Printf("%-20s %-30s %-10s %s\n", srv.Name, srv.Host, user, srv.Domain)
		}
		return nil
	},
}

var serverSSHCmd = &cobra.Command{
	Use:   "ssh <name>",
	Short: "SSH into a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		srv, err := config.LoadServer(name)
		if err != nil {
			return err
		}

		user := srv.User
		if user == "" {
			user = "root"
		}

		sshCmd := fmt.Sprintf("ssh %s@%s", user, srv.Host)
		if srv.SSHKey != "" {
			sshCmd = fmt.Sprintf("ssh -i %s %s@%s", srv.SSHKey, user, srv.Host)
		}
		fmt.Println(sshCmd)
		return nil
	},
}

// serverNameFromHost derives a server name from a host string.
// If name is provided, it is returned as-is.
// If host is an IP address (contains only digits and dots), the dots are replaced with dashes
// and the result is prefixed with "server-".
// Otherwise, the first segment before the first "." is returned.
func serverNameFromHost(host, name string) string {
	if name != "" {
		return name
	}
	// Check if host is an IP (no alpha characters)
	hasAlpha := false
	for _, r := range host {
		if unicode.IsLetter(r) {
			hasAlpha = true
			break
		}
	}
	if !hasAlpha {
		return "server-" + strings.ReplaceAll(host, ".", "-")
	}
	// Hostname: return first segment before "."
	parts := strings.SplitN(host, ".", 2)
	return parts[0]
}

// appNameToDBName converts an app name to a valid Postgres database/user name.
func appNameToDBName(appName string) string {
	return strings.ReplaceAll(appName, "-", "_")
}

// clientFromServer creates an agent.Client from a server config,
// using SSH alias if available, otherwise host/user/key.
func clientFromServer(srv *config.Server) *agent.Client {
	if srv.SSHAlias != "" {
		return agent.NewClientFromAlias(srv.SSHAlias)
	}
	return agent.NewClient(srv.Host, srv.User, srv.SSHKey)
}

const (
	caddyContainer = "ezkeel-caddy-1"
	caddyfilePath  = "/opt/ezkeel/Caddyfile"
	defaultAppPort = 80
)

// safeContainerName mirrors the agent's containerName logic for CLI-side use.
func safeContainerName(appName string) string {
	var b strings.Builder
	for _, r := range appName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return "ezkeel-" + b.String()
}

// addCaddyRoute appends a reverse proxy route to the remote Caddyfile and reloads Caddy.
func addCaddyRoute(client *agent.Client, domain, container string, port int) {
	ctx := context.Background()
	caddyEntry := fmt.Sprintf(`\n%s {\n    reverse_proxy %s:%d\n}\n`, domain, container, port)
	addCmd := fmt.Sprintf(`grep -q '%s' %s || printf '%s' >> %s`, domain, caddyfilePath, caddyEntry, caddyfilePath)
	if _, err := client.RunRemote(ctx, addCmd); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update Caddyfile: %v\n", err)
	}
	reloadCaddy(client)
}

// removeCaddyRoute removes a domain block from the remote Caddyfile and reloads Caddy.
func removeCaddyRoute(client *agent.Client, domain string) {
	ctx := context.Background()
	sedCmd := fmt.Sprintf(`sed -i '/%s/,/}/d' %s`, domain, caddyfilePath)
	client.RunRemote(ctx, sedCmd) //nolint:errcheck
	reloadCaddy(client)
}

func reloadCaddy(client *agent.Client) {
	ctx := context.Background()
	if _, err := client.RunRemote(ctx, fmt.Sprintf("docker exec %s caddy reload --config /etc/caddy/Caddyfile", caddyContainer)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: caddy reload failed: %v\n", err)
	}
}

// appPort returns the app's port, defaulting to 80 if unset.
func appPort(port int) int {
	if port == 0 {
		return defaultAppPort
	}
	return port
}

const bootstrapFlagName = "bootstrap"

// runnerForServer returns the bootstrap.Runner appropriate for srv:
// AliasRunner when SSHAlias is set, otherwise a Host/User/KeyFile-based
// SSHRunner. Mirrors clientFromServer's alias-vs-explicit fork.
func runnerForServer(srv *config.Server) bootstrap.Runner {
	if srv.SSHAlias != "" {
		return bootstrap.AliasRunner{Alias: srv.SSHAlias}
	}
	return bootstrap.SSHRunner{
		Host:    srv.Host,
		User:    srv.User,
		KeyFile: srv.SSHKey,
	}
}

// runBootstrap is a thin shim around bootstrap.Run so server_test.go
// can drive it with a fake runner.
func runBootstrap(ctx context.Context, runner bootstrap.Runner, opts bootstrap.Options) error {
	return bootstrap.Run(ctx, runner, opts)
}

func init() {
	serverAddCmd.Flags().String("host", "", "Server IP or hostname")
	serverAddCmd.Flags().String("name", "", "Server name")
	serverAddCmd.Flags().String("user", "root", "SSH user")
	serverAddCmd.Flags().String("key", "", "SSH private key path")
	serverAddCmd.Flags().String("domain", "", "Wildcard domain for apps")
	serverAddCmd.Flags().String("ssh-alias", "", "SSH config alias (e.g. 'hetzner') — uses your SSH config for proxy/key")
	serverAddCmd.Flags().Bool(bootstrapFlagName, true, "Install Docker + ezkeel-agent + Caddy networking on the server (set --bootstrap=false to skip)")
	serverAddCmd.Flags().Bool("hetzner", false, "Auto-provision a Hetzner Cloud VPS")
	serverAddCmd.Flags().String("hetzner-token", "", "Hetzner Cloud API token (or set HETZNER_TOKEN env)")
	serverAddCmd.Flags().String("hetzner-type", "cx22", "Hetzner server type (cx22=2vCPU/4GB, cx32=4vCPU/8GB)")
	serverAddCmd.Flags().String("hetzner-location", "fsn1", "Hetzner datacenter (fsn1, nbg1, hel1)")
	serverAddCmd.Flags().String("hetzner-ssh-key", "", "Name of SSH key in Hetzner Cloud console")
	serverCmd.AddCommand(serverAddCmd)
	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverSSHCmd)
}
