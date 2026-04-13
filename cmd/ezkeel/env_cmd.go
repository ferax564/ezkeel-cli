package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage app environment variables",
}

var envListCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List environment variables for an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		manifestPath := detect.ManifestPath(appName)

		m, err := detect.LoadManifest(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("app %q not found; run 'ezkeel apps' to list deployed apps", appName)
			}
			return fmt.Errorf("loading manifest for %q: %w", appName, err)
		}

		if len(m.Env) == 0 {
			fmt.Printf("No environment variables set for %q.\n", appName)
			fmt.Println("Use 'ezkeel env set <app> KEY=VALUE' to add one.")
			return nil
		}

		fmt.Printf("Environment variables for %q:\n\n", appName)
		for k, v := range m.Env {
			fmt.Printf("  %s=%s\n", k, v)
		}

		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set <app> KEY=VALUE [KEY=VALUE...]",
	Short: "Set environment variables for an app",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		pairs := args[1:]

		manifestPath := detect.ManifestPath(appName)

		m, err := detect.LoadManifest(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("app %q not found; run 'ezkeel apps' to list deployed apps", appName)
			}
			return fmt.Errorf("loading manifest for %q: %w", appName, err)
		}

		if m.Env == nil {
			m.Env = make(map[string]string)
		}

		for _, pair := range pairs {
			idx := strings.IndexByte(pair, '=')
			if idx < 0 {
				return fmt.Errorf("invalid format %q: expected KEY=VALUE", pair)
			}
			key := pair[:idx]
			value := pair[idx+1:]
			if key == "" {
				return fmt.Errorf("invalid format %q: key must not be empty", pair)
			}
			m.Env[key] = value
		}

		if err := m.Save(manifestPath); err != nil {
			return fmt.Errorf("saving manifest for %q: %w", appName, err)
		}

		fmt.Printf("Updated environment variables for %q.\n", appName)
		fmt.Println("Run 'ezkeel up' to apply changes.")
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envSetCmd)
}
