package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/spf13/cobra"
)

var pagesCmd = &cobra.Command{
	Use:   "pages",
	Short: "Manage static site hosting",
}

var pagesDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy static site to the platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		platformDir, _ := cmd.Flags().GetString("platform-dir")

		cfg := config.LoadGlobalConfig()
		host = config.FlagOrDefault(host, cfg.Platform.SSHHost)
		platformDir = config.FlagOrDefault(platformDir, cfg.Platform.PlatformDir)

		if host == "" {
			return fmt.Errorf("--host is required (SSH host for the VPS, e.g. hetzner)")
		}

		ws, err := loadWorkspaceFromDir(".")
		if err != nil {
			return fmt.Errorf("loading workspace.yaml: %w", err)
		}

		buildDir := ws.Pages.BuildDir
		if buildDir == "" {
			buildDir = "docs/"
		}

		if ws.Pages.BuildCmd != "" {
			fmt.Printf("Building: %s\n", ws.Pages.BuildCmd)
			buildCmd := exec.Command("sh", "-c", ws.Pages.BuildCmd)
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			if err := buildCmd.Run(); err != nil {
				return fmt.Errorf("build failed: %w", err)
			}
		}

		if _, err := os.Stat(buildDir); os.IsNotExist(err) {
			return fmt.Errorf("build directory %q does not exist", buildDir)
		}

		remotePath := buildPagesRemotePath(platformDir, ws.Name)
		fmt.Printf("Deploying %s to %s:%s\n", buildDir, host, remotePath)

		mkdirCmd := exec.Command("ssh", host, "mkdir", "-p", remotePath)
		if err := mkdirCmd.Run(); err != nil {
			return fmt.Errorf("creating remote directory: %w", err)
		}

		rsyncCmd := exec.Command("rsync", "-avz", "--delete", buildDir, host+":"+remotePath+"/")
		rsyncCmd.Stdout = os.Stdout
		rsyncCmd.Stderr = os.Stderr
		if err := rsyncCmd.Run(); err != nil {
			return fmt.Errorf("rsync failed: %w", err)
		}

		pageURL := fmt.Sprintf("https://%s/pages/%s/", pagesURLFromWorkspace(ws), ws.Name)
		fmt.Printf("\nDeployed! Site live at: %s\n", pageURL)
		return nil
	},
}

func buildPagesRemotePath(platformDir, project string) string {
	return filepath.Join(platformDir, "pages", project)
}

func pagesURLFromWorkspace(ws *config.Workspace) string {
	if ws.Pages.Domain != "" {
		return ws.Pages.Domain
	}
	return ws.Name + ".example.com"
}

func init() {
	pagesDeployCmd.Flags().String("host", "", "SSH host for the VPS (e.g. hetzner)")
	pagesDeployCmd.Flags().String("platform-dir", "/opt/ezkeel", "Platform install directory on VPS")
	pagesCmd.AddCommand(pagesDeployCmd)
}
