package main

import (
	"fmt"
	"os"

	"github.com/ferax564/ezkeel-cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ezkeel",
	Short: "The home for your coding agent — git push to your VPS",
	Long: `EZKeel is a self-hosted dev platform for coding agents.

It bundles Forgejo, Caddy, Docker, and an MCP server so that Claude Code,
Cursor, and Codex CLI work out of the box on your VPS. Push code, ezkeel
auto-detects the framework, builds an image, provisions databases, issues
TLS certs, and deploys — without GitHub Actions, without YAML.

Common workflows:

  Provision a fresh server          ezkeel server add --hetzner
  Deploy a repo                     ezkeel up <repo-url>
  Launch a coding agent on the box  ezkeel ai claude
  Stream container logs             ezkeel logs <app>
  Roll back to the previous deploy  ezkeel rollback <app>

Self-hosted, BYOV, $5/month VPS gets you a working stack. Docs at https://ezkeel.com/docs.html.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print ezkeel version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ezkeel v%s\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(platformCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(secretsCmd)
	rootCmd.AddCommand(aiCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(environmentCmd)
	rootCmd.AddCommand(ciCmd)
	rootCmd.AddCommand(pagesCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(domainCmd)
	rootCmd.AddCommand(agentCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
