/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"os"
	"path/filepath"
	"testing"
)

// withNodeNameFile points the lookup at a file under a temp directory, so a test
// can say what the machine knows about its own name without touching
// /var/lib/bashible.
func withNodeNameFile(t *testing.T, contents string, write bool) {
	t.Helper()

	original := nodeNamePath
	t.Cleanup(func() { nodeNamePath = original })

	path := filepath.Join(t.TempDir(), "discovered-node-name")
	if write {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write node name file: %v", err)
		}
	}
	nodeNamePath = path
}

func TestNodeNameComesFromTheFileBashiblePinsItIn(t *testing.T) {
	withNodeNameFile(t, "worker-rack3-07\n", true)

	got, err := DiscoverNodeName()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got != "worker-rack3-07" {
		t.Fatalf("got %q, want the pinned name", got)
	}
}

func TestNodeNameFallsBackToTheHostname(t *testing.T) {
	withNodeNameFile(t, "", false)

	got, err := DiscoverNodeName()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		t.Skipf("no hostname on this machine: %v", err)
	}
	if got != hostname {
		t.Fatalf("got %q, want the hostname %q", got, hostname)
	}
}

func TestAnEmptyNodeNameFileIsNotTakenAsAName(t *testing.T) {
	withNodeNameFile(t, "   \n", true)

	got, err := DiscoverNodeName()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got == "" {
		t.Fatal("an empty file produced an empty node name instead of falling back")
	}
}

func TestNodeNamePathIsWhereBashiblePinsIt(t *testing.T) {
	// The inhibitor is a systemd service, not a pod: there is no downward API to
	// ask, so it has to read the same file kubelet's --hostname-override comes
	// from. If that path ever moves, this is the copy that goes stale.
	if NodeNamePath != "/var/lib/bashible/discovered-node-name" {
		t.Fatalf("unexpected node name path %q", NodeNamePath)
	}
}
