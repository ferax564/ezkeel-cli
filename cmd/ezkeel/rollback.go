package main

import (
	"fmt"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <app>",
	Short: "Roll back an app to its previous deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		m, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		fmt.Printf("Rolling back %s to previous version...\n", appName)
		resp, err := client.Send(cmd.Context(), &agent.Request{
			Type: agent.CmdRollback,
			Rollback: &agent.RollbackRequest{
				AppName: appName,
				Port:    appPort(m.App.Port),
				Memory:  m.Resources.Memory,
				CPUs:    m.Resources.CPUs,
			},
		})
		if err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("agent error: %s", resp.Error)
		}

		fmt.Printf("%s %s rolled back successfully\n", tui.IconDone, appName)
		return nil
	},
}
