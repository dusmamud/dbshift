package ui_test

import (
	"strings"
	"testing"

	"github.com/dusmamud/dbshift/internal/ui"
)

func TestMaskURI(t *testing.T) {
	uri := "postgresql://myuser:supersecretpassword@example.com:26257/defaultdb?sslmode=verify-full"
	masked := ui.MaskURI(uri)

	if strings.Contains(masked, "supersecretpassword") {
		t.Errorf("MaskURI leaked raw password: %s", masked)
	}
	if !strings.Contains(masked, "******") {
		t.Errorf("expected masked asterisks, got: %s", masked)
	}
	if !strings.Contains(masked, "myuser") {
		t.Errorf("expected username preserved, got: %s", masked)
	}
}
