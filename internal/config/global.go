package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EzkeelHome returns the root ezkeel config directory.
// It honours the EZKEEL_HOME env var when set (used in tests), and falls
// back to ~/.ezkeel otherwise.
func EzkeelHome() string {
	if override := os.Getenv("EZKEEL_HOME"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ezkeel")
}

// GlobalConfig stores platform-wide defaults in ~/.ezkeel/config.yaml.
// Commands use these when flags are not provided.
type GlobalConfig struct {
	Platform PlatformConfig `yaml:"platform"`
}

// PlatformConfig holds connection details for the EZKeel platform.
type PlatformConfig struct {
	ForgejoURL            string `yaml:"forgejo_url"`
	ForgejoToken          string `yaml:"forgejo_token"`
	InfisicalURL          string `yaml:"infisical_url"`
	InfisicalClientID     string `yaml:"infisical_client_id"`
	InfisicalClientSecret string `yaml:"infisical_client_secret"`
	InfisicalOrg          string `yaml:"infisical_org"`
	SSHHost               string `yaml:"ssh_host"`
	PlatformDir           string `yaml:"platform_dir"`
	Owner                 string `yaml:"owner"`
}

// GlobalConfigPath returns the path to ~/.ezkeel/config.yaml.
func GlobalConfigPath() string {
	root := EzkeelHome()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "config.yaml")
}

// LoadGlobalConfig reads the global config from ~/.ezkeel/config.yaml.
// Returns a zero-value config if the file doesn't exist.
func LoadGlobalConfig() *GlobalConfig {
	cfg := &GlobalConfig{}
	path := GlobalConfigPath()
	if path == "" {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	yaml.Unmarshal(data, cfg)
	return cfg
}

// Save writes the global config to ~/.ezkeel/config.yaml.
func (c *GlobalConfig) Save() error {
	path := GlobalConfigPath()
	if path == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// FlagOrDefault returns the flag value if non-empty, otherwise the config default.
func FlagOrDefault(flag, configDefault string) string {
	if flag != "" {
		return flag
	}
	return configDefault
}
