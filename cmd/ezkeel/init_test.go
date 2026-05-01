package main

import "testing"

// TestValidProjectName mirrors the spec.validAppName allowlist so
// `ezkeel init` rejects names that `ezkeel up` would later reject —
// no silent scaffolding of an unusable spec.
func TestValidProjectName(t *testing.T) {
	valid := []string{"a", "my-app", "app1", "x-y-z", "abc", "todo-dashboard"}
	invalid := []string{
		"MyApp",         // uppercase
		"api_service",   // underscore
		"apps/api",      // slash
		"-leading",      // leading dash
		"trailing-",     // trailing dash
		"",              // empty
		"with space",    // space
		"app.name",      // dot
		"UPPER",         // all caps
	}
	for _, n := range valid {
		if !validProjectName(n) {
			t.Errorf("validProjectName(%q) = false, want true", n)
		}
	}
	for _, n := range invalid {
		if validProjectName(n) {
			t.Errorf("validProjectName(%q) = true, want false", n)
		}
	}

	// 64 chars (one over RFC 1123 hostname cap) is rejected; 63 chars
	// is the longest accepted name.
	max := make([]byte, 63)
	for i := range max {
		max[i] = 'a'
	}
	if !validProjectName(string(max)) {
		t.Errorf("validProjectName(63x'a') = false, want true (boundary)")
	}
	over := make([]byte, 64)
	for i := range over {
		over[i] = 'a'
	}
	if validProjectName(string(over)) {
		t.Errorf("validProjectName(64x'a') = true, want false (over RFC 1123 cap)")
	}
}
