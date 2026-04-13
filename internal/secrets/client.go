package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client wraps the Infisical CLI for a specific project.
type Client struct {
	projectID string
	domain    string // Infisical API URL (e.g. "https://secrets.ezkeel.com/api")
}

// NewClient returns a new Client for the given Infisical project ID and domain.
// If domain is empty, the Infisical CLI default is used.
func NewClient(projectID, domain string) *Client {
	return &Client{projectID: projectID, domain: domain}
}

// domainArgs returns --domain flag if configured.
func (c *Client) domainArgs() []string {
	if c.domain != "" {
		return []string{"--domain", c.domain}
	}
	return nil
}

// buildExportArgs constructs the CLI arguments for the `infisical export` command.
func (c *Client) buildExportArgs(env string) []string {
	args := []string{
		"export",
		"--projectId", c.projectID,
		"--env", env,
		"--format", "dotenv",
	}
	return append(args, c.domainArgs()...)
}

// buildRunArgs constructs the CLI arguments for the `infisical run` command.
// Multiple command parts are joined with a single space.
func (c *Client) buildRunArgs(env string, command ...string) []string {
	args := []string{
		"run",
		"--projectId", c.projectID,
		"--env", env,
		"--command", strings.Join(command, " "),
	}
	return append(args, c.domainArgs()...)
}

// ParseDotenv parses a dotenv-formatted string and returns a map of key/value pairs.
// Empty lines and lines starting with '#' are skipped. Each line is split on the
// first '=' character only, so values may contain '='.
func ParseDotenv(input string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(input, "\n") {
		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			// No '=' found — skip malformed line
			continue
		}
		key := trimmed[:idx]
		val := trimmed[idx+1:]
		// Strip surrounding single or double quotes from values
		if len(val) >= 2 && ((val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"')) {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	return result
}

// Export runs `infisical export` and returns the parsed key/value map.
func (c *Client) Export(env string) (map[string]string, error) {
	args := c.buildExportArgs(env)
	out, err := exec.Command("infisical", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("infisical export failed: %w", err)
	}
	return ParseDotenv(string(out)), nil
}

// InjectIntoShell calls Export and sets each secret as an environment variable
// in the current process via os.Setenv.
func (c *Client) InjectIntoShell(env string) error {
	secrets, err := c.Export(env)
	if err != nil {
		return err
	}
	for k, v := range secrets {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("os.Setenv(%q): %w", k, err)
		}
	}
	return nil
}

// Run executes the given command under `infisical run`, with stdio passed through
// to the current process (interactive-friendly).
func (c *Client) Run(env string, command ...string) error {
	args := c.buildRunArgs(env, command...)
	cmd := exec.Command("infisical", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("infisical run failed: %w", err)
	}
	return nil
}
