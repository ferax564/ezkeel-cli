package main

import "testing"

func TestBuildCloneURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		token   string
		project string
		want    string
	}{
		{
			name:    "no token",
			baseURL: "https://git.ezkeel.com",
			token:   "",
			project: "org/repo",
			want:    "https://git.ezkeel.com/org/repo",
		},
		{
			name:    "with token https",
			baseURL: "https://git.ezkeel.com",
			token:   "abc123",
			project: "org/repo",
			want:    "https://abc123@git.ezkeel.com/org/repo",
		},
		{
			name:    "with token http",
			baseURL: "http://localhost:3000",
			token:   "tok",
			project: "user/proj",
			want:    "http://tok@localhost:3000/user/proj",
		},
		{
			name:    "no scheme falls back to simple concat",
			baseURL: "git.ezkeel.com",
			token:   "tok",
			project: "repo",
			want:    "git.ezkeel.com/repo",
		},
		{
			name:    "trailing slash on base URL",
			baseURL: "https://git.ezkeel.com/",
			token:   "tok",
			project: "org/repo",
			want:    "https://tok@git.ezkeel.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCloneURL(tt.baseURL, tt.token, tt.project)
			if got != tt.want {
				t.Errorf("buildCloneURL(%q, %q, %q) = %q, want %q",
					tt.baseURL, tt.token, tt.project, got, tt.want)
			}
		})
	}
}
