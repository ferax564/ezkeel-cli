package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/pkg/forgejo"
	"github.com/ferax564/ezkeel-cli/internal/secrets"
	"github.com/spf13/cobra"
)

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Manage the EZKeel platform",
}

var platformInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the EZKeel platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		forgejoD, _ := cmd.Flags().GetString("forgejo-domain")
		infisicalD, _ := cmd.Flags().GetString("infisical-domain")

		// Check required tools
		if err := checkCommand("docker"); err != nil {
			return fmt.Errorf("docker is required: %w", err)
		}
		if err := checkCommand("docker", "compose", "version"); err != nil {
			return fmt.Errorf("docker compose is required: %w", err)
		}

		// Create platform directory
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating platform dir %q: %w", dir, err)
		}

		// Generate .env content with all required secrets
		envContent, err := generateEnvContent(forgejoD, infisicalD)
		if err != nil {
			return fmt.Errorf("generating env content: %w", err)
		}

		// Write .env file
		envPath := filepath.Join(dir, ".env")
		if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
			return fmt.Errorf("writing .env: %w", err)
		}
		fmt.Printf("Written %s\n", envPath)

		// Parse env content into a vars map for template substitution
		vars := secrets.ParseDotenv(envContent)

		// Copy top-level platform files
		platformFiles := []string{"docker-compose.yml", "Caddyfile"}
		for _, f := range platformFiles {
			if err := copyPlatformFile(f, dir, vars); err != nil {
				// Non-fatal: platform files may not exist in all environments
				fmt.Fprintf(os.Stderr, "warning: could not copy platform file %q: %v\n", f, err)
			}
		}

		// Copy forgejo sub-directory files
		forgejoDir := filepath.Join(dir, "forgejo")
		if err := os.MkdirAll(forgejoDir, 0o755); err != nil {
			return fmt.Errorf("creating forgejo config dir: %w", err)
		}
		if err := copyPlatformFile(filepath.Join("forgejo", "app.ini.template"), dir, vars); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not copy forgejo/app.ini.template: %v\n", err)
		}

		// Run docker compose up -d
		// Create the external `ezkeel-apps` network up front. Compose
		// declares it as `external: true` so it can attach caddy to it
		// (so caddy can reach <name>.apps.ezkeel.com upstream containers
		// the agent spawns). Without this pre-creation, `docker compose
		// up -d` fails with "network ezkeel-apps not found".
		// `docker network create` is idempotent if we ignore "already
		// exists" — checking first is the cleanest pattern.
		netCheck := exec.Command("docker", "network", "inspect", "ezkeel-apps")
		if err := netCheck.Run(); err != nil {
			netCreate := exec.Command("docker", "network", "create", "ezkeel-apps")
			netCreate.Stdout = os.Stdout
			netCreate.Stderr = os.Stderr
			if err := netCreate.Run(); err != nil {
				return fmt.Errorf("docker network create ezkeel-apps: %w", err)
			}
		}

		composeCmd := exec.Command("docker", "compose", "--project-directory", dir, "up", "-d")
		composeCmd.Stdout = os.Stdout
		composeCmd.Stderr = os.Stderr
		if err := composeCmd.Run(); err != nil {
			return fmt.Errorf("docker compose up -d: %w", err)
		}

		fmt.Printf("\nEZKeel platform installed at %s\n", dir)
		fmt.Printf("Forgejo:   https://%s\n", forgejoD)
		fmt.Printf("Infisical: https://%s\n", infisicalD)
		fmt.Printf("Admin password stored in %s\n", envPath)

		fmt.Println()
		fmt.Print(formatSetupGuide(vars))
		return nil
	},
}

var platformSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Print post-install setup guide for admin accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		forgejoURL, _ := cmd.Flags().GetString("forgejo-url")
		forgejoToken, _ := cmd.Flags().GetString("forgejo-token")

		cfg := config.LoadGlobalConfig()
		forgejoURL = config.FlagOrDefault(forgejoURL, cfg.Platform.ForgejoURL)
		forgejoToken = config.FlagOrDefault(forgejoToken, cfg.Platform.ForgejoToken)

		envPath := filepath.Join(dir, ".env")
		data, err := os.ReadFile(envPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w (run 'ezkeel platform install' first)", envPath, err)
		}
		vars := secrets.ParseDotenv(string(data))
		fmt.Print(formatSetupGuide(vars))

		// Auto-register the runner if Forgejo credentials are provided.
		if forgejoURL != "" && forgejoToken != "" {
			fmt.Println("\nRegistering Forgejo Actions runner...")
			client := forgejo.NewClient(forgejoURL, forgejoToken)
			runnerToken, err := client.CreateRunnerRegistrationToken()
			if err != nil {
				return fmt.Errorf("creating runner registration token: %w", err)
			}

			// Replace RUNNER_TOKEN in the .env file.
			envContent := string(data)
			oldLine := "RUNNER_TOKEN=" + vars["RUNNER_TOKEN"]
			newLine := "RUNNER_TOKEN=" + runnerToken
			envContent = strings.ReplaceAll(envContent, oldLine, newLine)
			if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
				return fmt.Errorf("updating .env with runner token: %w", err)
			}
			fmt.Printf("Runner token written to %s\n", envPath)

			// Restart the runner container so it picks up the new token.
			composePath := filepath.Join(dir, "docker-compose.yml")
			restartCmd := exec.Command("docker", "compose", "-f", composePath, "restart", "runner")
			restartCmd.Stdout = os.Stdout
			restartCmd.Stderr = os.Stderr
			if err := restartCmd.Run(); err != nil {
				return fmt.Errorf("restarting runner container: %w", err)
			}
			fmt.Println("Runner container restarted with new registration token.")
		}

		return nil
	},
}

func init() {
	platformInstallCmd.Flags().String("dir", "/opt/ezkeel", "Directory to install the platform into")
	platformInstallCmd.Flags().String("forgejo-domain", "git.ezkeel.com", "Domain for the Forgejo instance")
	platformInstallCmd.Flags().String("infisical-domain", "secrets.ezkeel.com", "Domain for the Infisical instance")
	platformCmd.AddCommand(platformInstallCmd)

	platformSetupCmd.Flags().String("dir", "/opt/ezkeel", "Platform install directory")
	platformSetupCmd.Flags().String("forgejo-url", "", "Forgejo base URL (e.g. https://git.example.com)")
	platformSetupCmd.Flags().String("forgejo-token", "", "Forgejo admin API token for runner registration")
	platformCmd.AddCommand(platformSetupCmd)
}

// checkCommand verifies that a command is available and runnable.
func checkCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// generateSecret produces a cryptographically random hex string of n bytes.
func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// secretSpec defines a secret to generate for the .env file.
type secretSpec struct {
	key  string
	size int
}

// platformSecrets lists all secrets generated for the platform .env file.
var platformSecrets = []secretSpec{
	{"POSTGRES_PASSWORD", 32},
	{"FORGEJO_SECRET_KEY", 32},
	{"FORGEJO_INTERNAL_TOKEN", 32},
	{"INFISICAL_ENCRYPTION_KEY", 16},
	{"INFISICAL_AUTH_SECRET", 32},
	{"RUNNER_TOKEN", 20},
	{"ADMIN_PASSWORD", 16},
}

// generateEnvContent generates all required secrets and returns the full .env
// content as a string.
func generateEnvContent(forgejoD, infisicalD string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "FORGEJO_DOMAIN=%s\n", forgejoD)
	fmt.Fprintf(&b, "INFISICAL_DOMAIN=%s\n", infisicalD)

	for _, s := range platformSecrets {
		val, err := generateSecret(s.size)
		if err != nil {
			return "", fmt.Errorf("generating %s: %w", s.key, err)
		}
		fmt.Fprintf(&b, "%s=%s\n", s.key, val)
	}

	return b.String(), nil
}

// copyPlatformFile copies a file from the bundled platform/ directory into dir,
// replacing template variables along the way.
func copyPlatformFile(name, dir string, vars map[string]string) error {
	// Try several candidate source locations.
	candidates := []string{
		filepath.Join("platform", name),
		filepath.Join("/opt/ezkeel/platform", name),
	}

	var data []byte
	var readErr error
	for _, src := range candidates {
		data, readErr = os.ReadFile(src)
		if readErr == nil {
			break
		}
	}
	if readErr != nil {
		return readErr
	}

	content := replaceTemplateVars(string(data), vars)
	dst := filepath.Join(dir, name)
	// Ensure parent directory exists (needed for sub-path names like forgejo/app.ini.template)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating parent dir for %q: %w", dst, err)
	}
	// #nosec G703 -- not a path-traversal vulnerability: name is hardcoded by the platform install flow (curated list of known template paths), not user-supplied. CLI runs as the operator on their own machine; threat model has no untrusted source for this argument.
	return os.WriteFile(dst, []byte(content), 0o644)
}

// replaceTemplateVars substitutes {{KEY}} placeholders in a template string
// using the provided vars map.
func replaceTemplateVars(s string, vars map[string]string) string {
	for key, value := range vars {
		s = strings.ReplaceAll(s, "{{"+key+"}}", value)
	}
	return s
}

// formatSetupGuide returns a formatted post-install setup guide populated with
// values from the parsed .env vars map.
func formatSetupGuide(vars map[string]string) string {
	forgejoD := vars["FORGEJO_DOMAIN"]
	infisicalD := vars["INFISICAL_DOMAIN"]
	adminPass := vars["ADMIN_PASSWORD"]

	return fmt.Sprintf(`EZKeel Platform — Post-Install Setup
=====================================

Step 1: Create Forgejo admin account
  Open:     https://%s
  Username: ezkeel-admin
  Password: %s
  Email:    admin@%s

  After logging in:
  a) Go to Site Administration > User Accounts to manage users
  b) Go to Site Administration > Runners to register the CI runner
     (the runner token is in your .env file as RUNNER_TOKEN)

Step 2: Create Infisical admin account
  Open:     https://%s/signup
  Use any email/password for the admin account.

  After logging in:
  a) Create an organization (e.g. "ezkeel")
  b) Create a project for each EZKeel project you want to manage
  c) Add secrets (e.g. ANTHROPIC_API_KEY) to the dev environment
  d) Generate a service token or machine identity for CLI access:
     infisical login

Step 3: Verify the platform
  Test Forgejo:   curl -s https://%s/api/v1/version
  Test Infisical: curl -s https://%s/api/v1/auth/check-auth

Done! You can now run:
  ezkeel init <project> --forgejo-url https://%s --forgejo-token <token>
`, forgejoD, adminPass, forgejoD,
		infisicalD,
		forgejoD, infisicalD, forgejoD)
}
