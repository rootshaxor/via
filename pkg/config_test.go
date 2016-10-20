package via

import (
	"testing"
)

func TestBranch(t *testing.T) {
	var (
		expect = "linux-x86_64"
		got, _ = config.Branch()
	)
	if expect != got {
		t.Errorf("expected '%s' got '%s'.", expect, got)
	}
}

func TestConfig(t *testing.T) {
	if config == nil {
		t.Errorf("config is nil")
	}
}
