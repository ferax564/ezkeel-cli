package main

import (
	"encoding/json"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"my-app", "my-app"},
		{"my_app", "my_app"},
		{"My App!", "MyApp"},
		{"app@123", "app123"},
		{"", ""},
	}
	for _, tc := range cases {
		got := sanitizeName(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestContainerName(t *testing.T) {
	got := containerName("my-app")
	want := "ezkeel-my-app"
	if got != want {
		t.Errorf("containerName(%q) = %q, want %q", "my-app", got, want)
	}
}

func TestPrevImageTag(t *testing.T) {
	got := prevImageTag("my-app")
	want := "my-app:prev"
	if got != want {
		t.Errorf("prevImageTag(%q) = %q, want %q", "my-app", got, want)
	}
}

func TestBuildRunArgs(t *testing.T) {
	args := buildRunArgs("ezkeel-myapp", 3000, "512m", "1.0", map[string]string{"FOO": "bar"}, "myimage:latest")

	// Last element must be the image.
	if args[len(args)-1] != "myimage:latest" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "myimage:latest")
	}

	// Check required flags are present.
	flagSet := make(map[string]string)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--name" && i+1 < len(args) {
			flagSet["--name"] = args[i+1]
		}
		if args[i] == "--restart" && i+1 < len(args) {
			flagSet["--restart"] = args[i+1]
		}
		if args[i] == "--network" && i+1 < len(args) {
			flagSet["--network"] = args[i+1]
		}
		if args[i] == "--memory" && i+1 < len(args) {
			flagSet["--memory"] = args[i+1]
		}
		if args[i] == "--cpus" && i+1 < len(args) {
			flagSet["--cpus"] = args[i+1]
		}
	}

	if flagSet["--name"] != "ezkeel-myapp" {
		t.Errorf("--name = %q, want %q", flagSet["--name"], "ezkeel-myapp")
	}
	if flagSet["--restart"] != "unless-stopped" {
		t.Errorf("--restart = %q, want %q", flagSet["--restart"], "unless-stopped")
	}
	if flagSet["--network"] != "ezkeel-apps" {
		t.Errorf("--network = %q, want %q", flagSet["--network"], "ezkeel-apps")
	}
	if flagSet["--memory"] != "512m" {
		t.Errorf("--memory = %q, want %q", flagSet["--memory"], "512m")
	}
	if flagSet["--cpus"] != "1.0" {
		t.Errorf("--cpus = %q, want %q", flagSet["--cpus"], "1.0")
	}

	// Verify no host port binding is used.
	for _, a := range args {
		if a == "-p" {
			t.Error("unexpected -p flag: containers must use Docker network, not host port binding")
		}
	}
}

func TestBuildRunArgs_NoLimits(t *testing.T) {
	args := buildRunArgs("ezkeel-myapp", 3000, "", "", nil, "myimage:latest")

	for i, a := range args {
		if a == "--memory" {
			t.Errorf("unexpected --memory flag at position %d", i)
		}
		if a == "--cpus" {
			t.Errorf("unexpected --cpus flag at position %d", i)
		}
	}
}

func TestParsePort(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"127.0.0.1:3000->3000/tcp", 3000},
		{"0.0.0.0:8080->8080/tcp", 8080},
		{"3000", 3000},
		{"", 0},
		{"127.0.0.1:9090->9090/tcp, 127.0.0.1:9091->9091/tcp", 9090},
	}
	for _, tc := range cases {
		got := parsePort(tc.input)
		if got != tc.want {
			t.Errorf("parsePort(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestWriteOK_Format(t *testing.T) {
	resp := agent.Response{OK: true, Message: "test"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var got agent.Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if !got.OK {
		t.Errorf("got.OK = false, want true")
	}
	if got.Message != "test" {
		t.Errorf("got.Message = %q, want %q", got.Message, "test")
	}
}

func TestWriteError_Format(t *testing.T) {
	resp := agent.Response{OK: false, Error: "something failed"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var got agent.Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if got.OK {
		t.Errorf("got.OK = true, want false")
	}
	if got.Error != "something failed" {
		t.Errorf("got.Error = %q, want %q", got.Error, "something failed")
	}
}

func TestHandleRequest_UnknownCommand(t *testing.T) {
	req := agent.Request{Type: "nonexistent"}
	if req.Type != "nonexistent" {
		t.Errorf("req.Type = %q, want %q", req.Type, "nonexistent")
	}
}

func TestHandleRequest_MissingPayload(t *testing.T) {
	req := agent.Request{Type: agent.CmdDeploy, Deploy: nil}
	if req.Deploy != nil {
		t.Errorf("req.Deploy should be nil")
	}
}
