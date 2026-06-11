package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/spf13/cobra"
)

// appJSON is the machine-readable shape emitted by `ezkeel apps --json`.
type appJSON struct {
	Name      string   `json:"name"`
	Server    string   `json:"server"`
	Framework string   `json:"framework"`
	Port      int      `json:"port"`
	URL       string   `json:"url"`
	Domains   []string `json:"domains"`
}

// appsToJSON maps stored manifests to the --json output shape. Domains
// is never null — agents iterate it without a nil check.
func appsToJSON(manifests []*detect.AppManifest) []appJSON {
	out := make([]appJSON, 0, len(manifests))
	for _, m := range manifests {
		domains := m.Domains
		if domains == nil {
			domains = []string{}
		}
		out = append(out, appJSON{
			Name:      m.Name,
			Server:    m.Server,
			Framework: m.App.Framework,
			Port:      m.App.Port,
			URL:       "https://" + m.Domain,
			Domains:   domains,
		})
	}
	return out
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List deployed apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		appsDir := filepath.Join(config.EzkeelHome(), "apps")

		entries, err := os.ReadDir(appsDir)
		if err != nil {
			if os.IsNotExist(err) {
				if jsonOut {
					return emitJSON(os.Stdout, []appJSON{})
				}
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

		if jsonOut {
			return emitJSON(os.Stdout, appsToJSON(manifests))
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

// logsJSON is the machine-readable shape emitted by `ezkeel logs --json`.
type logsJSON struct {
	App   string   `json:"app"`
	Lines []string `json:"lines"`
}

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "Stream logs from a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		jsonOut, _ := cmd.Flags().GetBool("json")
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

		if jsonOut {
			logLines := resp.Logs
			if logLines == nil {
				logLines = []string{}
			}
			return emitJSON(os.Stdout, logsJSON{App: appName, Lines: logLines})
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
	appsCmd.Flags().Bool("json", false, "Output machine-readable JSON")
	logsCmd.Flags().Int("lines", 100, "Number of log lines to show")
	logsCmd.Flags().Bool("json", false, "Output machine-readable JSON")
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
