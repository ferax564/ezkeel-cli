package main

import "testing"

func TestCheckResult_String(t *testing.T) {
	tests := []struct {
		cr   checkResult
		want string
	}{
		{checkResult{Name: "SSH", OK: true, Detail: "connected"}, "SSH"},
		{checkResult{Name: "Docker", OK: false, Detail: "not installed"}, "Docker"},
	}

	for _, tt := range tests {
		if tt.cr.Name != tt.want {
			t.Errorf("Name = %q, want %q", tt.cr.Name, tt.want)
		}
		if tt.cr.OK && tt.cr.Detail != "connected" {
			t.Errorf("Detail = %q", tt.cr.Detail)
		}
	}
}

func TestParseDiskUsage(t *testing.T) {
	output := "/dev/sda1       39G   12G   25G  33% /"
	pct, err := parseDiskUsagePct(output)
	if err != nil {
		t.Fatalf("parseDiskUsagePct error: %v", err)
	}
	if pct != 33 {
		t.Errorf("pct = %d, want 33", pct)
	}
}

func TestParseDiskUsage_BadInput(t *testing.T) {
	_, err := parseDiskUsagePct("bad input")
	if err == nil {
		t.Error("expected error for bad input")
	}
}
