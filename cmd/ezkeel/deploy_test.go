package main

import "testing"

func TestBuildImageName(t *testing.T) {
	tests := []struct {
		name       string
		forgejoURL string
		owner      string
		project    string
		want       string
	}{
		{
			name:       "standard",
			forgejoURL: "https://git.ezkeel.com",
			owner:      "ezkeel-admin",
			project:    "my-app",
			want:       "git.ezkeel.com/ezkeel-admin/my-app:latest",
		},
		{
			name:       "with port",
			forgejoURL: "https://git.example.com:3000",
			owner:      "org",
			project:    "api",
			want:       "git.example.com:3000/org/api:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildImageName(tt.forgejoURL, tt.owner, tt.project)
			if got != tt.want {
				t.Errorf("buildImageName() = %q, want %q", got, tt.want)
			}
		})
	}
}
