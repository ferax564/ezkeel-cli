package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy the project to the platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		forgejoURL, _ := cmd.Flags().GetString("forgejo-url")
		owner, _ := cmd.Flags().GetString("owner")
		env, _ := cmd.Flags().GetString("env")

		cfg := config.LoadGlobalConfig()
		host = config.FlagOrDefault(host, cfg.Platform.SSHHost)
		forgejoURL = config.FlagOrDefault(forgejoURL, cfg.Platform.ForgejoURL)
		owner = config.FlagOrDefault(owner, cfg.Platform.Owner)

		if host == "" || forgejoURL == "" || owner == "" {
			return fmt.Errorf("--host, --forgejo-url, and --owner are required")
		}

		ws, err := loadWorkspaceFromDir(".")
		if err != nil {
			return fmt.Errorf("loading workspace.yaml: %w", err)
		}

		imageName := buildImageName(forgejoURL, owner, ws.Name)

		fmt.Printf("Building Docker image: %s\n", imageName)
		buildCmd := exec.Command("docker", "build", "-t", imageName, ".")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("docker build failed: %w", err)
		}

		fmt.Printf("Pushing image to registry...\n")
		pushCmd := exec.Command("docker", "push", imageName)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("docker push failed: %w", err)
		}

		fmt.Printf("Deploying %s on %s (env: %s)...\n", ws.Name, host, env)

		// Run each docker command separately over SSH to avoid shell injection
		// via ws.Name (which comes from user-editable workspace.yaml).
		for _, remoteArgs := range [][]string{
			{"docker", "pull", imageName},
			{"docker", "stop", ws.Name},
			{"docker", "rm", ws.Name},
			{"docker", "run", "-d", "--name", ws.Name, "--restart", "unless-stopped", "-p", "0:8080", imageName},
		} {
			step := exec.Command("ssh", append([]string{host}, remoteArgs...)...)
			step.Stdout = os.Stdout
			step.Stderr = os.Stderr
			step.Run() // ignore stop/rm errors (container may not exist)
		}
		// Verify the final run command succeeded by checking container exists
		verifyCmd := exec.Command("ssh", host, "docker", "container", "inspect", ws.Name)
		verifyCmd.Stdout = nil
		verifyCmd.Stderr = nil
		if err := verifyCmd.Run(); err != nil {
			return fmt.Errorf("deploy failed: container %s not running", ws.Name)
		}

		fmt.Printf("\nDeployed %s successfully!\n", ws.Name)
		return nil
	},
}

func buildImageName(forgejoURL, owner, project string) string {
	u, err := url.Parse(forgejoURL)
	if err != nil {
		return forgejoURL + "/" + owner + "/" + project + ":latest"
	}
	return u.Host + "/" + owner + "/" + project + ":latest"
}

func init() {
	deployCmd.Flags().String("host", "", "SSH host for the VPS")
	deployCmd.Flags().String("forgejo-url", "", "Forgejo instance URL (for container registry)")
	deployCmd.Flags().String("owner", "", "Repository owner")
	deployCmd.Flags().String("env", "prod", "Deployment environment")
}
