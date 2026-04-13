package detect

import (
	"os"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"gopkg.in/yaml.v3"
)

// Resources describes container resource limits.
type Resources struct {
	Memory string `yaml:"memory,omitempty"` // e.g. "512m"
	CPUs   string `yaml:"cpus,omitempty"`   // e.g. "1.0"
}

// AppManifest stores the persisted configuration for a deployed application.
// It is saved to ~/.ezkeel/apps/<name>.yaml.
type AppManifest struct {
	Name      string                   `yaml:"name"`
	Repo      string                   `yaml:"repo"`
	Server    string                   `yaml:"server"`
	App       AppConfig                `yaml:"app"`
	Services  map[string]ServiceConfig `yaml:"services,omitempty"`
	Env       map[string]string        `yaml:"env,omitempty"`
	Domain    string                   `yaml:"domain"`
	Domains   []string                 `yaml:"domains,omitempty"`
	Resources Resources                `yaml:"resources,omitempty"`
}

// AppConfig holds the application's framework and runtime settings.
type AppConfig struct {
	Framework  string `yaml:"framework"`
	Build      string `yaml:"build,omitempty"`
	Start      string `yaml:"start,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	Dockerfile string `yaml:"dockerfile"`
}

// ServiceConfig describes a backing service (e.g. a database) used by the app.
type ServiceConfig struct {
	Version  string `yaml:"version"`
	Database string `yaml:"database"`
}

// ManifestPath returns the path to the manifest file for the given app name.
// The file is located at <EzkeelHome>/apps/<name>.yaml.
func ManifestPath(appName string) string {
	return filepath.Join(config.EzkeelHome(), "apps", appName+".yaml")
}

// Save marshals the manifest to YAML and writes it to path with 0600 permissions.
// Parent directories are created as needed.
func (m *AppManifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadManifest reads and parses the YAML manifest at path.
// Returns an error if the file does not exist or cannot be parsed.
func LoadManifest(path string) (*AppManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m AppManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
