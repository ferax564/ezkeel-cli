package environment

import (
	"os"
	"os/exec"

	"github.com/ferax564/ezkeel-cli/internal/preflight"
)

// buildArgs constructs devcontainer CLI arguments for build or up commands.
func buildArgs(action, projectDir string) []string {
	return []string{action, "--workspace-folder", projectDir}
}

// execArgs constructs devcontainer CLI arguments for exec commands.
func execArgs(projectDir string, command ...string) []string {
	args := []string{"exec", "--workspace-folder", projectDir}
	return append(args, command...)
}

// CheckDevcontainerCLI verifies the devcontainer CLI is available.
func CheckDevcontainerCLI() error {
	return preflight.CheckCommand("devcontainer",
		"Install: npm install -g @devcontainers/cli")
}

// BuildDevContainer runs "devcontainer build" for the given project directory.
func BuildDevContainer(projectDir string) error {
	if err := CheckDevcontainerCLI(); err != nil {
		return err
	}
	cmd := exec.Command("devcontainer", buildArgs("build", projectDir)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartDevContainer runs "devcontainer up" for the given project directory.
func StartDevContainer(projectDir string) error {
	if err := CheckDevcontainerCLI(); err != nil {
		return err
	}
	cmd := exec.Command("devcontainer", buildArgs("up", projectDir)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecInDevContainer runs a command inside the dev container.
func ExecInDevContainer(projectDir string, command ...string) error {
	if err := CheckDevcontainerCLI(); err != nil {
		return err
	}
	cmd := exec.Command("devcontainer", execArgs(projectDir, command...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
