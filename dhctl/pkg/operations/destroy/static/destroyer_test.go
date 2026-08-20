// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package static

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

const connectedHost = "10.0.0.1"

// abortDestroyer builds a destroyer the way the abort path does: no Prepare, so no node user and no
// API-discovered master IPs. availableHosts defaults to the single host --ssh-host would give.
// cleaned counts the cleanup script per host.
func abortDestroyer(t *testing.T, cachedHosts map[string]string, availableHosts ...string) (*Destroyer, map[string]int) {
	t.Helper()

	stateCache := cache.NewTestCache()
	if cachedHosts != nil {
		require.NoError(t, dhctlstate.SaveMasterHosts(t.Context(), stateCache, cachedHosts))
	}

	cleaned := make(map[string]int)

	if len(availableHosts) == 0 {
		availableHosts = []string{connectedHost}
	}

	available := make([]session.Host, 0, len(availableHosts))
	for _, host := range availableHosts {
		available = append(available, session.Host{Host: host})
	}

	input := session.Input{
		User:           "user",
		Port:           "22",
		AvailableHosts: available,
	}
	sshProvider := testssh.NewSSHProvider(session.NewSession(input), true)

	for _, host := range append([]string{connectedHost}, slices.Collect(maps.Values(cachedHosts))...) {
		sshProvider.AddCommandProvider(host, func(_ testssh.Bastion, scriptPath string, _ ...string) *testssh.Command {
			if !strings.Contains(scriptPath, "cleanup_static_node.sh") {
				return nil
			}

			return testssh.NewCommand([]byte("ok")).WithRun(func() { cleaned[host]++ })
		})
	}

	destroyer := NewDestroyer(&DestroyerParams{
		SSHClientProvider: sshProvider,
		State:             NewDestroyState(stateCache),
		Logger:            dhlog.Discard(),
		Loops:             LoopsParams{DestroyMaster: retry.NewEmptyParams()},
	})

	return destroyer, cleaned
}

// TestDestroyerAbortCleansEveryCachedHost is the point of reading the cache at all: the user aborts
// naming the one master they can reach, and the other two have to be cleaned up too. With no
// --ssh-host the connection is opened over every host the cache lists instead, so all of them are
// excluded as connected, nothing lands in additionalMastersHosts, and the tail loop is the only
// thing left that can reach the masters other than the one the session happened to pick.
func TestDestroyerAbortCleansEveryCachedHost(t *testing.T) {
	for name, availableHosts := range map[string][]string{
		"only the master the user named is reachable": nil,
		"every master is reachable":                   {connectedHost, "10.0.0.2", "10.0.0.3"},
	} {
		t.Run(name, func(t *testing.T) {
			destroyer, cleaned := abortDestroyer(t, map[string]string{
				"first-master": connectedHost,
				"master-1":     "10.0.0.2",
				"master-2":     "10.0.0.3",
			}, availableHosts...)

			require.NoError(t, destroyer.destroyCluster(t.Context(), true))
			require.Equal(t, map[string]int{connectedHost: 1, "10.0.0.2": 1, "10.0.0.3": 1}, cleaned)
		})
	}
}

// A bootstrap that stops before PostInfraPreflights never records its hosts, and that is the most
// common thing an abort undoes. It cleans up what it was given, and nothing else.
func TestDestroyerAbortWithoutCachedHostsCleansTheGivenHost(t *testing.T) {
	destroyer, cleaned := abortDestroyer(t, nil)

	require.NoError(t, destroyer.destroyCluster(t.Context(), true))
	require.Equal(t, map[string]int{connectedHost: 1}, cleaned)
}

// TestDestroyerAbortRefusesCacheOfAnotherCluster guards the mine every static cluster shares: one
// cache identity for all of them, so a cache that does not mention the connected host describes
// somebody else's masters.
func TestDestroyerAbortRefusesCacheOfAnotherCluster(t *testing.T) {
	destroyer, cleaned := abortDestroyer(t, map[string]string{
		"first-master": "10.9.9.1",
		"master-1":     "10.9.9.2",
	})

	err := destroyer.destroyCluster(t.Context(), true)
	require.ErrorContains(t, err, "another cluster")
	require.Empty(t, cleaned)
}
