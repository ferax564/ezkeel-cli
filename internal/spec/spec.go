// Package spec parses ezkeel.yaml — the declarative deploy spec read by
// `ezkeel up`. The format is versioned via a `# spec: ezkeel/v1` stamp on
// the first non-blank line so future breaking changes can be rejected
// loudly instead of silently misparsed.
package spec

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Stamp is the required first-line marker. Bumped only for breaking changes;
// additive fields stay on v1.
const Stamp = "# spec: ezkeel/v1"

// Spec is the v1 deploy spec parsed from ezkeel.yaml.
type Spec struct {
	Name      string             `yaml:"name"`
	Framework string             `yaml:"framework,omitempty"`
	Build     string             `yaml:"build,omitempty"`
	Start     string             `yaml:"start,omitempty"`
	Port      int                `yaml:"port,omitempty"`
	Services  map[string]Service `yaml:"services,omitempty"`
	Runtime   string             `yaml:"runtime,omitempty"` // "docker" (default) or "sysbox"
	Sandbox   bool               `yaml:"sandbox,omitempty"`
	Env       []string           `yaml:"env,omitempty"`
	Resources Resources          `yaml:"resources,omitempty"`
}

// Service describes a backing service the app needs.
type Service struct {
	Engine  string `yaml:"engine"`
	Version string `yaml:"version,omitempty"`
}

// Resources mirrors AppManifest.Resources for symmetry.
type Resources struct {
	Memory string `yaml:"memory,omitempty"`
	CPUs   string `yaml:"cpus,omitempty"`
}

// Load reads and parses ezkeel.yaml at the given path. It enforces the
// `# spec: ezkeel/v1` stamp on the first non-blank line so future
// versions can be rejected loudly instead of silently misparsed.
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	if err := requireStamp(string(data)); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	if s.Name == "" {
		return nil, fmt.Errorf("%s: name is required", path)
	}
	if s.Runtime == "" {
		s.Runtime = "docker"
	}
	return &s, nil
}

// Find walks up from startDir looking for ezkeel.yaml, returning the
// first absolute path found or an error at the filesystem root.
func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory: %w", err)
	}
	for {
		candidate := filepath.Join(dir, "ezkeel.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("ezkeel.yaml not found")
		}
		dir = parent
	}
}

func requireStamp(body string) error {
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == Stamp {
			return nil
		}
		if strings.HasPrefix(line, "# spec: ezkeel/") {
			return fmt.Errorf("unsupported spec version (this CLI understands %q)", Stamp)
		}
		return fmt.Errorf("missing %q stamp on first line", Stamp)
	}
	return fmt.Errorf("empty file (expected %q stamp)", Stamp)
}
