package bootstrap

import (
	"strings"
	"testing"
)

// The line has to be usable as printed: an operator copies it, and a wrong
// port or a missing quote costs them the debugging session the line exists to
// prevent.
func TestBastionForwardLineShape(t *testing.T) {
	line := buildBastionForwardLine("ubuntu", "185.120.186.26", 0, "10.12.2.16", "/tmp/dhctl/admin.kubeconfig")
	t.Logf("line: %s", line)
	for _, want := range []string{
		"ssh -f -N -L 6445:10.12.2.16:6443 ubuntu@185.120.186.26",
		"https://10.12.2.16:6443",
		"https://127.0.0.1:6445",
		"/tmp/dhctl/admin.kubeconfig",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("missing %q in %q", want, line)
		}
	}
}
