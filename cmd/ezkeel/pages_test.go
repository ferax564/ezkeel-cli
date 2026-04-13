package main

import "testing"

func TestBuildPagesRemotePath(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		project string
		want    string
	}{
		{"default", "/opt/ezkeel", "my-app", "/opt/ezkeel/pages/my-app"},
		{"custom", "/srv/platform", "docs-site", "/srv/platform/pages/docs-site"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPagesRemotePath(tt.dir, tt.project)
			if got != tt.want {
				t.Errorf("buildPagesRemotePath(%q, %q) = %q, want %q", tt.dir, tt.project, got, tt.want)
			}
		})
	}
}
