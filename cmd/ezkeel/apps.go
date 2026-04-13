package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/spf13/cobra"
)

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List deployed apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		appsDir := filepath.Join(config.EzkeelHome(), "apps")

		entries, err := os.ReadDir(appsDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("%s\n\nNo apps deployed yet. Run 'ezkeel up' to deploy your first app.\n", tui.Banner())
				return nil
			}
			return fmt.Errorf("reading apps directory: %w", err)
		}

		// Filter for .yaml files
		var manifests []*detect.AppManifest
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			path := filepath.Join(appsDir, entry.Name())
			m, err := detect.LoadManifest(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
				continue
			}
			manifests = append(manifests, m)
		}

		fmt.Printf("%s\n\n", tui.Banner())

		if len(manifests) == 0 {
			fmt.Println("No apps deployed yet. Run 'ezkeel up' to deploy your first app.")
			return nil
		}

		fmt.Printf("%-20s %-15s %s\n", "NAME", "FRAMEWORK", "DOMAIN")
		fmt.Printf("%-20s %-15s %s\n", "----", "---------", "------")
		for _, m := range manifests {
			fmt.Printf("%s %-20s %-15s %s\n", tui.IconLive, m.Name, m.App.Framework, m.Domain)
		}

		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "Stream logs from a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		_, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		lines, _ := cmd.Flags().GetInt("lines")
		resp, err := client.Send(cmd.Context(), &agent.Request{
			Type: agent.CmdLogs,
			Logs: &agent.LogsRequest{AppName: appName, Lines: lines},
		})
		if err != nil {
			return fmt.Errorf("fetching logs: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("agent error: %s", resp.Error)
		}

		for _, line := range resp.Logs {
			fmt.Println(line)
		}
		return nil
	},
}

// resolveApp loads the manifest and creates an agent client for the given app.
func resolveApp(appName string) (*detect.AppManifest, *agent.Client, error) {
	m, err := detect.LoadManifest(detect.ManifestPath(appName))
	if err != nil {
		return nil, nil, fmt.Errorf("app %q not found; run 'ezkeel apps' to list deployed apps", appName)
	}
	srv, err := config.LoadServer(m.Server)
	if err != nil {
		return nil, nil, fmt.Errorf("loading server %q: %w", m.Server, err)
	}
	return m, clientFromServer(srv), nil
}

func init() {
	logsCmd.Flags().Int("lines", 100, "Number of log lines to show")
}

var downCmd = &cobra.Command{
	Use:   "down <app>",
	Short: "Stop and remove a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		_, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		resp, err := client.Send(cmd.Context(), &agent.Request{
			Type: agent.CmdStop,
			Stop: &agent.StopRequest{AppName: appName},
		})
		if err != nil {
			return fmt.Errorf("stopping app: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("agent error: %s", resp.Error)
		}

		// Remove local manifest
		if err := os.Remove(detect.ManifestPath(appName)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove manifest: %v\n", err)
		}

		fmt.Printf("App %q stopped and removed.\n", appName)
		return nil
	},
}
