package main

import "testing"

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		domain string
		valid  bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"my-app.example.com", true},
		{"", false},
		{"example", false},
		{".example.com", false},
		{"example..com", false},
		{"exa mple.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := isValidDomain(tt.domain)
			if got != tt.valid {
				t.Errorf("isValidDomain(%q) = %v, want %v", tt.domain, got, tt.valid)
			}
		})
	}
}
