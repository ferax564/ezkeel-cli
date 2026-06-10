package agent

import (
	"fmt"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("vps.example.com", "deploy", "/home/user/.ssh/id_ed25519")

	if c.Host != "vps.example.com" {
		t.Errorf("Host: got %q, want %q", c.Host, "vps.example.com")
	}
	if c.User != "deploy" {
		t.Errorf("User: got %q, want %q", c.User, "deploy")
	}
	if c.SSHKey != "/home/user/.ssh/id_ed25519" {
		t.Errorf("SSHKey: got %q, want %q", c.SSHKey, "/home/user/.ssh/id_ed25519")
	}
}

func TestClient_BuildSSHArgs(t *testing.T) {
	c := NewClient("vps.example.com", "deploy", "/home/user/.ssh/id_ed25519")
	args := c.sshArgs("ezkeel-agent", "--request")

	// Verify user@host is present.
	userAtHost := "deploy@vps.example.com"
	found := false
	for _, a := range args {
		if a == userAtHost {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sshArgs: expected %q in args %v", userAtHost, args)
	}

	// Verify -i key is present.
	keyFound := false
	for i, a := range args {
		if a == "-i" && i+1 < len(args) && args[i+1] == "/home/user/.ssh/id_ed25519" {
			keyFound = true
			break
		}
	}
	if !keyFound {
		t.Errorf("sshArgs: expected \"-i /home/user/.ssh/id_ed25519\" in args %v", args)
	}

	// Verify remote command is appended at the end.
	if len(args) < 2 {
		t.Fatal("sshArgs: too few arguments")
	}
	last := args[len(args)-1]
	secondLast := args[len(args)-2]
	if secondLast != "ezkeel-agent" || last != "--request" {
		t.Errorf("sshArgs: expected remote cmd at end, got %q %q", secondLast, last)
	}
}

func TestStampProtocolVersion(t *testing.T) {
	req := &Request{Type: CmdStatus}
	stampProtocolVersion(req)
	if req.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("ProtocolVersion: got %d, want %d", req.ProtocolVersion, CurrentProtocolVersion)
	}

	// An explicit version set by the caller is preserved.
	req = &Request{Type: CmdStatus, ProtocolVersion: 42}
	stampProtocolVersion(req)
	if req.ProtocolVersion != 42 {
		t.Errorf("ProtocolVersion: got %d, want caller's 42", req.ProtocolVersion)
	}
}

func TestWarnOnVersionSkew(t *testing.T) {
	var warnings []string
	c := &Client{Warn: func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}}

	// Older agent (including pre-versioning version 0) triggers a warning.
	c.warnOnVersionSkew(&Response{OK: true, ProtocolVersion: 0})
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one skew warning", warnings)
	}

	// Same or newer protocol stays silent.
	warnings = nil
	c.warnOnVersionSkew(&Response{OK: true, ProtocolVersion: CurrentProtocolVersion})
	c.warnOnVersionSkew(&Response{OK: true, ProtocolVersion: CurrentProtocolVersion + 1})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}
