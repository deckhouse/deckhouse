/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package systemd

import (
	"path/filepath"
	"strings"
	"testing"
)

// logind merges *.conf.d drop-ins sorted by file name (strcmp on the basename) across all search
// directories, and for a single-value option such as InhibitDelayMaxSec the last file wins. The
// directory is consulted only to break ties between identical names, so our drop-in in /etc does
// not automatically beat a vendor drop-in in /usr/lib: it wins only if its name sorts last. If it
// does not, logind keeps a smaller InhibitDelayMaxSec than we wrote and node shutdown is no longer
// delayed.
func TestDropInNameSortsLast(t *testing.T) {
	// Real drop-ins shipped by Debian/Ubuntu packages and by kubelet itself.
	competitors := []string{
		"99-kubelet.conf",                          // kubelet, InhibitDelayMaxSec=<shutdownGracePeriod>
		"unattended-upgrades-logind-maxdelay.conf", // unattended-upgrades, InhibitDelayMaxSec=30
		"ec2-hibinit-agent-ignore-powerkey.conf",   // ec2-hibinit-agent
		"sxmo-utils.conf",                          // sxmo-utils
		"mobile-tweaks.conf",                       // mobile-tweaks-common
	}

	for _, name := range competitors {
		if d8ShutdownInhibitorConf <= name {
			t.Errorf("drop-in %q does not sort after %q: logind would apply %q last and our InhibitDelayMaxSec would be overridden",
				d8ShutdownInhibitorConf, name, name)
		}
	}

	if !strings.HasSuffix(d8ShutdownInhibitorConf, ".conf") {
		t.Errorf("drop-in %q must end in .conf or logind ignores it", d8ShutdownInhibitorConf)
	}
	if filepath.Base(d8ShutdownInhibitorConf) != d8ShutdownInhibitorConf {
		t.Errorf("drop-in %q must be a bare file name", d8ShutdownInhibitorConf)
	}
}
