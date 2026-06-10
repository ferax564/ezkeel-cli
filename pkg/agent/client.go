package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Client sends commands to a remote VPS over SSH.
type Client struct {
	Host     string
	User     string
	SSHKey   string
	SSHAlias string // if set, uses "ssh <alias>" — respects full SSH config

	// Warn receives human-readable compatibility warnings (e.g. the
	// agent speaks an older protocol version than this client). Nil
	// means write to stderr.
	Warn func(format string, args ...any)
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
//
// Send stamps the request with CurrentProtocolVersion (unless the caller
// already set one) and warns via c.Warn when the responding agent speaks
// an older protocol than this client.
func (c *Client) Send(ctx context.Context, req *Request) (*Response, error) {
	stampProtocolVersion(req)
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
	c.warnOnVersionSkew(&resp)
	return &resp, nil
}

// Version probes the agent's version and protocol version without side
// effects. Agents predating protocol versioning answer with an "unknown
// command type" error response; callers should treat that as protocol
// version 0.
func (c *Client) Version(ctx context.Context) (*Response, error) {
	return c.Send(ctx, &Request{Type: CmdVersion})
}

// stampProtocolVersion fills in the request's protocol version when the
// caller didn't set one explicitly.
func stampProtocolVersion(req *Request) {
	if req.ProtocolVersion == 0 {
		req.ProtocolVersion = CurrentProtocolVersion
	}
}

// warnOnVersionSkew emits a warning when the agent that produced resp
// speaks an older protocol than this client. A newer agent responding
// to an older client is not warned about here — the agent stays
// backward-compatible with old requests by construction.
func (c *Client) warnOnVersionSkew(resp *Response) {
	if resp.ProtocolVersion >= CurrentProtocolVersion {
		return
	}
	warn := c.Warn
	if warn == nil {
		warn = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
		}
	}
	warn("remote ezkeel-agent speaks protocol version %d (client speaks %d) — update it with `ezkeel server update-agent`",
		resp.ProtocolVersion, CurrentProtocolVersion)
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
