package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Client sends commands to a remote VPS over SSH.
type Client struct {
	Host     string
	User     string
	SSHKey   string
	SSHAlias string // if set, uses "ssh <alias>" — respects full SSH config
}

// NewClient returns a new Client targeting the given host.
func NewClient(host, user, sshKey string) *Client {
	return &Client{Host: host, User: user, SSHKey: sshKey}
}

// NewClientFromAlias returns a Client that uses an SSH config alias.
func NewClientFromAlias(alias string) *Client {
	return &Client{SSHAlias: alias}
}

// sshArgs builds the argument list for an ssh invocation.
func (c *Client) sshArgs(remoteCmd ...string) []string {
	if c.SSHAlias != "" {
		args := []string{c.SSHAlias}
		args = append(args, remoteCmd...)
		return args
	}
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if c.SSHKey != "" {
		args = append(args, "-i", c.SSHKey)
	}
	args = append(args, c.User+"@"+c.Host)
	args = append(args, remoteCmd...)
	return args
}

// Send marshals req, pipes it to "ezkeel-agent --request" on the remote host
// via SSH, and parses the JSON response. The context controls the SSH process
// lifetime — if the context is canceled, the SSH process is killed.
func (c *Client) Send(ctx context.Context, req *Request) (*Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	args := c.sshArgs("ezkeel-agent", "--request")
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = bytes.NewReader(data)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh agent call: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}

// UploadImage streams a Docker image to the remote host using "docker save | ssh docker load".
// The image is piped directly without buffering in memory.
// If onProgress is non-nil, it is called periodically with the cumulative bytes transferred.
func (c *Client) UploadImage(imageTag string, onProgress func(int64)) error {
	saveCmd := exec.Command("docker", "save", imageTag)
	loadCmd := exec.Command("ssh", c.sshArgs("docker", "load")...)

	pipe, err := saveCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}

	cr := &CountingReader{Reader: pipe, OnProgress: onProgress}
	loadCmd.Stdin = cr

	if err := loadCmd.Start(); err != nil {
		return fmt.Errorf("starting ssh docker load: %w", err)
	}
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("docker save %s: %w", imageTag, err)
	}
	if err := loadCmd.Wait(); err != nil {
		return fmt.Errorf("docker load on remote: %w", err)
	}
	return nil
}

// RunRemote executes an arbitrary command on the remote host and returns its
// combined stdout+stderr output. The context controls the SSH process lifetime.
func (c *Client) RunRemote(ctx context.Context, command string) (string, error) {
	args := c.sshArgs(command)
	cmd := exec.CommandContext(ctx, "ssh", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("remote command %q: %w", command, err)
	}
	return string(out), nil
}
