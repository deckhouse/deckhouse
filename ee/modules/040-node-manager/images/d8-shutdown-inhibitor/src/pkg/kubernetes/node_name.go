/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

// nodeNamePath is a variable rather than the constant itself so a test can point
// the lookup somewhere other than the real machine.
var nodeNamePath = NodeNamePath

// DiscoverNodeName is the name of the Node this inhibitor acts on.
//
// It cannot be the machine's hostname: a node registers under kubelet's
// --hostname-override, which Deckhouse lets an operator set to something else, and
// an inhibitor that guessed wrong would cordon nothing and hold up a shutdown
// waiting on a node that does not exist. This runs as a systemd service rather
// than a pod, so there is no downward API to ask; the name is read from the file
// bashible pins it in, the same one kubelet is started from.
//
// The hostname stays as the fallback, for a machine whose file is missing - an
// older node that has not run bashible since this was introduced.
func DiscoverNodeName() (string, error) {
	data, err := os.ReadFile(nodeNamePath)
	switch {
	case err == nil:
		if name := strings.TrimSpace(string(data)); name != "" {
			return name, nil
		}
		dlog.Warn("the node name file is empty, falling back to the hostname",
			slog.String("path", nodeNamePath))
	case !os.IsNotExist(err):
		return "", fmt.Errorf("read %s: %w", nodeNamePath, err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("no node name file and the hostname is unreadable: %w", err)
	}

	return hostname, nil
}
