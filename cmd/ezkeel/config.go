package main

import (
	"fmt"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global EZKeel configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current global configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadGlobalConfig()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
		fmt.Print(string(data))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a global configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		cfg := config.LoadGlobalConfig()

		switch key {
		case "forgejo_url":
			cfg.Platform.ForgejoURL = value
		case "forgejo_token":
			cfg.Platform.ForgejoToken = value
		case "infisical_url":
			cfg.Platform.InfisicalURL = value
		case "ssh_host":
			cfg.Platform.SSHHost = value
		case "platform_dir":
			cfg.Platform.PlatformDir = value
		case "owner":
			cfg.Platform.Owner = value
		default:
			return fmt.Errorf("unknown config key: %s", key)
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}
