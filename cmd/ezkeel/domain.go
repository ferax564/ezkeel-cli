package main

import (
	"fmt"
	"strings"

	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/spf13/cobra"
)

// isValidDomain checks if a string looks like a valid domain name.
// Only allows alphanumeric characters, hyphens, and dots.
func isValidDomain(d string) bool {
	if d == "" || strings.HasPrefix(d, ".") || strings.Contains(d, "..") {
		return false
	}
	parts := strings.Split(d, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage custom domains for deployed apps",
}

var domainAddCmd = &cobra.Command{
	Use:   "add <app> <domain>",
	Short: "Add a custom domain to an app",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		domain := args[1]

		if !isValidDomain(domain) {
			return fmt.Errorf("invalid domain: %q", domain)
		}

		m, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		// Check for duplicates.
		for _, d := range m.Domains {
			if d == domain {
				fmt.Printf("Domain %q already configured for %s.\n", domain, appName)
				return nil
			}
		}

		m.Domains = append(m.Domains, domain)
		if err := m.Save(detect.ManifestPath(appName)); err != nil {
			return fmt.Errorf("saving manifest: %w", err)
		}

		addCaddyRoute(client, domain, safeContainerName(appName), appPort(m.App.Port))

		fmt.Printf("%s Domain %q added to %s\n", tui.IconDone, domain, appName)
		fmt.Printf("\nPoint %s to your server IP with a DNS A record.\n", domain)
		fmt.Println("Caddy will auto-provision the SSL certificate.")
		return nil
	},
}

var domainRemoveCmd = &cobra.Command{
	Use:   "remove <app> <domain>",
	Short: "Remove a custom domain from an app",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		domain := args[1]

		m, client, err := resolveApp(appName)
		if err != nil {
			return err
		}

		found := false
		var updated []string
		for _, d := range m.Domains {
			if d == domain {
				found = true
				continue
			}
			updated = append(updated, d)
		}
		if !found {
			return fmt.Errorf("domain %q not found on app %s", domain, appName)
		}

		m.Domains = updated
		if err := m.Save(detect.ManifestPath(appName)); err != nil {
			return fmt.Errorf("saving manifest: %w", err)
		}

		removeCaddyRoute(client, domain)

		fmt.Printf("%s Domain %q removed from %s\n", tui.IconDone, domain, appName)
		return nil
	},
}

var domainListCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List custom domains for an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		manifestPath := detect.ManifestPath(appName)
		m, err := detect.LoadManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("app %q not found", appName)
		}

		fmt.Printf("Domains for %s:\n\n", appName)
		fmt.Printf("  %s (auto)\n", m.Domain)
		for _, d := range m.Domains {
			fmt.Printf("  %s (custom)\n", d)
		}
		return nil
	},
}

func init() {
	domainCmd.AddCommand(domainAddCmd)
	domainCmd.AddCommand(domainRemoveCmd)
	domainCmd.AddCommand(domainListCmd)
}
