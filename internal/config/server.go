package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Server holds connection details for a remote deployment target.
type Server struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	User     string `yaml:"user,omitempty"`
	SSHKey   string `yaml:"ssh_key,omitempty"`
	SSHAlias string `yaml:"ssh_alias,omitempty"` // SSH config alias (e.g. "hetzner") — uses full SSH config
	Domain   string `yaml:"domain"`
}

// ServersDir returns the directory where server configs are stored (~/.ezkeel/servers/).
func ServersDir() string {
	root := EzkeelHome()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "servers")
}

// SaveServer writes a server config to ~/.ezkeel/servers/<name>.yaml with 0600 permissions.
// User defaults to "root" when not set.
func SaveServer(srv *Server) error {
	if srv.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if srv.User == "" {
		srv.User = "root"
	}

	dir := ServersDir()
	if dir == "" {
		return fmt.Errorf("could not determine servers directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(srv)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, srv.Name+".yaml")
	return os.WriteFile(path, data, 0o600)
}

// LoadServer reads a server config from ~/.ezkeel/servers/<name>.yaml.
func LoadServer(name string) (*Server, error) {
	dir := ServersDir()
	if dir == "" {
		return nil, fmt.Errorf("could not determine servers directory")
	}

	path := filepath.Join(dir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("server %q not found", name)
		}
		return nil, err
	}

	var srv Server
	if err := yaml.Unmarshal(data, &srv); err != nil {
		return nil, err
	}
	return &srv, nil
}

// ListServers returns all server configs stored in ~/.ezkeel/servers/.
func ListServers() ([]*Server, error) {
	dir := ServersDir()
	if dir == "" {
		return nil, fmt.Errorf("could not determine servers directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Server{}, nil
		}
		return nil, err
	}

	var servers []*Server
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		serverName := strings.TrimSuffix(name, ".yaml")
		srv, err := LoadServer(serverName)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

// DefaultServer returns the first server found in ~/.ezkeel/servers/.
// Returns an error if no servers are configured.
func DefaultServer() (*Server, error) {
	servers, err := ListServers()
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers configured; run 'ezkeel config set' to add one")
	}
	return servers[0], nil
}
