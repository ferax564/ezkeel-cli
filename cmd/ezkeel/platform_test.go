package main

import (
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/secrets"
)

// TestGenerateEnvContent verifies that all required keys are present in the
// generated .env content.
func TestGenerateEnvContent(t *testing.T) {
	content, err := generateEnvContent("git.example.com", "secrets.example.com")
	if err != nil {
		t.Fatalf("generateEnvContent returned error: %v", err)
	}

	requiredKeys := []string{
		"FORGEJO_DOMAIN",
		"INFISICAL_DOMAIN",
		"POSTGRES_PASSWORD",
		"FORGEJO_SECRET_KEY",
		"FORGEJO_INTERNAL_TOKEN",
		"INFISICAL_ENCRYPTION_KEY",
		"INFISICAL_AUTH_SECRET",
		"RUNNER_TOKEN",
		"ADMIN_PASSWORD",
	}

	for _, key := range requiredKeys {
		if !strings.Contains(content, key+"=") {
			t.Errorf("missing required key %q in generated .env content", key)
		}
	}

	// Verify domain values are present
	if !strings.Contains(content, "FORGEJO_DOMAIN=git.example.com") {
		t.Error("FORGEJO_DOMAIN value not set correctly")
	}
	if !strings.Contains(content, "INFISICAL_DOMAIN=secrets.example.com") {
		t.Error("INFISICAL_DOMAIN value not set correctly")
	}
}

// TestGenerateEnvContentSecretsAreUnique verifies that all generated secret
// values are unique and non-empty.
func TestGenerateEnvContentSecretsAreUnique(t *testing.T) {
	content, err := generateEnvContent("git.example.com", "secrets.example.com")
	if err != nil {
		t.Fatalf("generateEnvContent returned error: %v", err)
	}

	vars := secrets.ParseDotenv(content)

	secretKeys := []string{
		"POSTGRES_PASSWORD",
		"FORGEJO_SECRET_KEY",
		"FORGEJO_INTERNAL_TOKEN",
		"INFISICAL_ENCRYPTION_KEY",
		"INFISICAL_AUTH_SECRET",
		"RUNNER_TOKEN",
		"ADMIN_PASSWORD",
	}

	// Check each secret is non-empty
	for _, key := range secretKeys {
		val, ok := vars[key]
		if !ok {
			t.Errorf("secret key %q missing from parsed vars", key)
			continue
		}
		if val == "" {
			t.Errorf("secret key %q has empty value", key)
		}
	}

	// Check all secret values are unique
	seen := make(map[string]string) // value -> first key that had it
	for _, key := range secretKeys {
		val := vars[key]
		if prev, exists := seen[val]; exists {
			t.Errorf("secret values are not unique: %q and %q share value %q", prev, key, val)
		}
		seen[val] = key
	}
}

// TestFormatSetupGuide verifies that the setup guide contains all expected
// domain values, passwords, and step headings.
func TestFormatSetupGuide(t *testing.T) {
	vars := map[string]string{
		"FORGEJO_DOMAIN":   "git.example.com",
		"INFISICAL_DOMAIN": "secrets.example.com",
		"ADMIN_PASSWORD":   "test-password-123",
	}

	guide := formatSetupGuide(vars)

	checks := []string{
		"git.example.com",
		"secrets.example.com",
		"test-password-123",
		"Step 1",
		"Step 2",
		"Step 3",
		"Forgejo",
		"Infisical",
	}
	for _, check := range checks {
		if !strings.Contains(guide, check) {
			t.Errorf("formatSetupGuide() missing %q", check)
		}
	}
}

// TestReplaceTemplateVars verifies that {{KEY}} placeholders are substituted
// with values from the vars map.
func TestReplaceTemplateVars(t *testing.T) {
	vars := map[string]string{
		"FORGEJO_DOMAIN":    "git.example.com",
		"POSTGRES_PASSWORD": "supersecret",
	}

	template := `DOMAIN = {{FORGEJO_DOMAIN}}
PASSWD = {{POSTGRES_PASSWORD}}
OTHER  = {{UNKNOWN_KEY}}
`
	got := replaceTemplateVars(template, vars)

	if !strings.Contains(got, "DOMAIN = git.example.com") {
		t.Errorf("FORGEJO_DOMAIN not substituted; got:\n%s", got)
	}
	if !strings.Contains(got, "PASSWD = supersecret") {
		t.Errorf("POSTGRES_PASSWORD not substituted; got:\n%s", got)
	}
	// Unknown keys should remain unchanged
	if !strings.Contains(got, "{{UNKNOWN_KEY}}") {
		t.Errorf("unknown key should remain unsubstituted; got:\n%s", got)
	}
}
