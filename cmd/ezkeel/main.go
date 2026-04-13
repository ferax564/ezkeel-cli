package main

import (
	"fmt"
	"os"

	"github.com/ferax564/ezkeel-cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ezkeel",
	Short: "Deploy any repo to your server in one command",
	Long:  "EZKeel auto-detects your framework, builds a Docker image, and deploys to your VPS with SSL. Self-hosted for $5/month.",
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
