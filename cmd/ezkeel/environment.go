package main

import (
	"fmt"
	"os"

	"github.com/ferax564/ezkeel-cli/internal/environment"
	"github.com/spf13/cobra"
)

var environmentCmd = &cobra.Command{
	Use:     "environment",
	Aliases: []string{"env"},
	Short:   "Manage the project's dev container environment",
}

var envBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the dev container image",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := resolveProjectDir(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("Building dev container for %s...\n", dir)
		return environment.BuildDevContainer(dir)
	},
}

var envStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the dev container",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := resolveProjectDir(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("Starting dev container for %s...\n", dir)
		return environment.StartDevContainer(dir)
	},
}

var envExecCmd = &cobra.Command{
	Use:   "exec -- <command...>",
	Short: "Execute a command inside the dev container",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := resolveProjectDir(cmd)
		if err != nil {
			return err
		}
		return environment.ExecInDevContainer(dir, args...)
	},
}

// resolveProjectDir returns the project directory from the --dir flag or cwd.
func resolveProjectDir(cmd *cobra.Command) (string, error) {
	dir, _ := cmd.Flags().GetString("dir")
	if dir != "" {
		return dir, nil
	}
	return os.Getwd()
}

func init() {
	environmentCmd.PersistentFlags().String("dir", "", "Project directory (default: current directory)")
	environmentCmd.AddCommand(envBuildCmd)
	environmentCmd.AddCommand(envStartCmd)
	environmentCmd.AddCommand(envExecCmd)
}
