package main

import (
	"testing"
)

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
