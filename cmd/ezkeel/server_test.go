package main

import (
	"context"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/bootstrap"
)

type recordingRunner struct{ cmds []string }

func (r *recordingRunner) Run(_ context.Context, cmd string) ([]byte, error) {
	r.cmds = append(r.cmds, cmd)
	return nil, nil
}

func TestRunBootstrapInvokesPackage(t *testing.T) {
	rec := &recordingRunner{}
	if err := runBootstrap(context.Background(), rec, bootstrap.Options{AgentURL: "https://example/agent"}); err != nil {
		t.Fatalf("runBootstrap: %v", err)
	}
	if len(rec.cmds) < 3 {
		t.Fatalf("calls = %d, want >=3", len(rec.cmds))
	}
}

func TestServerNameFromHost(t *testing.T) {
	tests := []struct {
		host string
		name string
		want string
	}{
		{"168.119.1.1", "", "server-168-119-1-1"},
		{"168.119.1.1", "vps01", "vps01"},
		{"my-server.example.com", "", "my-server"},
		{"192.168.0.10", "", "server-192-168-0-10"},
		{"192.168.0.10", "myvps", "myvps"},
		{"simple", "", "simple"},
		{"sub.domain.tld", "", "sub"},
	}

	for _, tt := range tests {
		t.Run(tt.host+"/"+tt.name, func(t *testing.T) {
			got := serverNameFromHost(tt.host, tt.name)
			if got != tt.want {
				t.Errorf("serverNameFromHost(%q, %q) = %q, want %q", tt.host, tt.name, got, tt.want)
			}
		})
	}
}
