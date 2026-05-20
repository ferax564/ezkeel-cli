package main

import "testing"

func TestEnvCommandResolvesAppEnvironmentCommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"env"})
	if err != nil {
		t.Fatalf("Find(env) error: %v", err)
	}
	if cmd != envCmd {
		t.Fatalf("Find(env) = %q, want app env command", cmd.CommandPath())
	}
}

func TestEnvironmentCommandStillResolves(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"environment"})
	if err != nil {
		t.Fatalf("Find(environment) error: %v", err)
	}
	if cmd != environmentCmd {
		t.Fatalf("Find(environment) = %q, want environment command", cmd.CommandPath())
	}
}
