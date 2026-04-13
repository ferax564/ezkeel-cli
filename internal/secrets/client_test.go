package secrets

import (
	"reflect"
	"testing"
)

func TestBuildExportArgs(t *testing.T) {
	c := Client{projectID: "proj-123", domain: "https://secrets.example.com/api"}
	got := c.buildExportArgs("dev")
	want := []string{"export", "--projectId", "proj-123", "--env", "dev", "--format", "dotenv", "--domain", "https://secrets.example.com/api"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildExportArgs() = %v, want %v", got, want)
	}
}

func TestBuildExportArgsNoDomain(t *testing.T) {
	c := Client{projectID: "proj-123"}
	got := c.buildExportArgs("dev")
	want := []string{"export", "--projectId", "proj-123", "--env", "dev", "--format", "dotenv"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildExportArgs() = %v, want %v", got, want)
	}
}

func TestBuildRunArgs(t *testing.T) {
	c := Client{projectID: "proj-123", domain: "https://secrets.example.com/api"}
	got := c.buildRunArgs("dev", "claude", "do something")
	want := []string{"run", "--projectId", "proj-123", "--env", "dev", "--command", "claude do something", "--domain", "https://secrets.example.com/api"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildRunArgs() = %v, want %v", got, want)
	}
}

func TestParseDotenv(t *testing.T) {
	input := `ANTHROPIC_API_KEY=sk-ant-123
DATABASE_URL=postgres://localhost/db
# comment line
EMPTY_VAL=

REDIS_URL=redis://localhost:6379
`
	got := ParseDotenv(input)

	cases := []struct {
		key  string
		want string
	}{
		{"ANTHROPIC_API_KEY", "sk-ant-123"},
		{"DATABASE_URL", "postgres://localhost/db"},
		{"EMPTY_VAL", ""},
		{"REDIS_URL", "redis://localhost:6379"},
	}

	for _, tc := range cases {
		val, ok := got[tc.key]
		if !ok {
			t.Errorf("key %q not found in parsed result", tc.key)
			continue
		}
		if val != tc.want {
			t.Errorf("got[%q] = %q, want %q", tc.key, val, tc.want)
		}
	}

	// Ensure comment line is not a key
	if _, ok := got["# comment line"]; ok {
		t.Error("comment line should not be parsed as a key")
	}
	if _, ok := got["#"]; ok {
		t.Error("comment line starting with # should not appear as key")
	}
}
