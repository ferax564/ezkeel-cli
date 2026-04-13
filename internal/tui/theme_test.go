package tui

import "testing"

func TestThemeColors(t *testing.T) {
	if Green == "" {
		t.Error("Green should not be empty")
	}
	if Brand == "" {
		t.Error("Brand should not be empty")
	}
}

func TestFormatBanner(t *testing.T) {
	if Banner() == "" {
		t.Error("Banner should not be empty")
	}
}
