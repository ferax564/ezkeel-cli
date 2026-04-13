package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup <app>",
	Short: "Backup the database for a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		m, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		if len(m.Services) == 0 {
			return fmt.Errorf("app %q has no database service configured", appName)
		}

		dbName := appNameToDBName(appName)

		fmt.Printf("Backing up database %q...\n", dbName)
		resp, err := client.Send(cmd.Context(), &agent.Request{
			Type:     agent.CmdDBBackup,
			DBBackup: &agent.DBBackupRequest{Database: dbName},
		})
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("agent error: %s", resp.Error)
		}

		outDir, _ := cmd.Flags().GetString("output")
		timestamp := time.Now().Format("20060102-150405")
		filename := filepath.Join(outDir, fmt.Sprintf("%s-%s.sql", dbName, timestamp))

		dump := ""
		if len(resp.Logs) > 0 {
			dump = resp.Logs[0]
		}

		if err := os.WriteFile(filename, []byte(dump), 0o600); err != nil {
			return fmt.Errorf("writing backup file: %w", err)
		}

		fmt.Printf("Backup saved to %s (%s)\n", filename, resp.Message)
		return nil
	},
}

func init() {
	backupCmd.Flags().String("output", ".", "Directory to save the backup file")
}
