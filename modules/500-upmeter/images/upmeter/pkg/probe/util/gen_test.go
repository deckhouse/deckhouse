package util

import (
	"testing"
)

func Test_AgentUniqueId(t *testing.T) {
	a, b := AgentUniqueId(), AgentUniqueId()
	if a != b {
		t.Errorf("expected %q == %q", a, b)
	}
}

func Test_RandomIdentifier(t *testing.T) {
	prefix := "upmeter-test-object"
	a, b := RandomIdentifier(prefix), RandomIdentifier(prefix)
	if a == b {
		t.Errorf("expected %q != %q", a, b)
	}
}
