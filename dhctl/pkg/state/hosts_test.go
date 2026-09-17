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

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
)

// A master a converge created is kept out of the live session on purpose — one session
// carries one generation of users — so the hosts cache is the only place its address lives.
// Dropping it maps the node to an empty address, which the readiness check refuses.
func TestMergeMasterHosts(t *testing.T) {
	sessionHosts := []session.Host{
		{Host: "10.0.0.1", Name: "cluster-master-0"},
		{Host: "10.0.0.2", Name: "cluster-master-1"},
	}
	cachedHosts := []session.Host{
		{Host: "10.0.0.9", Name: "cluster-master-1"},
		{Host: "10.0.0.3", Name: "cluster-master-2"},
	}

	require.Equal(t, []session.Host{
		{Host: "10.0.0.1", Name: "cluster-master-0"},
		{Host: "10.0.0.9", Name: "cluster-master-1"},
		{Host: "10.0.0.3", Name: "cluster-master-2"},
	}, MergeMasterHosts(sessionHosts, cachedHosts))
}

// One writer of the hosts cache stores a master whose SSH address came back empty. Letting
// that entry win hides the address the session has, and a node with no address is a node
// nothing can be cleaned up on.
func TestMergeMasterHostsKeepsAnAddressOverAnEmptyOne(t *testing.T) {
	sessionHosts := []session.Host{{Host: "10.0.0.1", Name: "cluster-master-0"}}
	cachedHosts := []session.Host{
		{Host: "", Name: "cluster-master-0"},
		{Host: "", Name: "cluster-master-1"},
	}

	require.Equal(t, []session.Host{
		{Host: "10.0.0.1", Name: "cluster-master-0"},
	}, MergeMasterHosts(sessionHosts, cachedHosts))
}

// The addresses of --ssh-host carry no node name, and the hosts cache is empty until a
// master is recreated. The node's own infrastructure state is what names the rest of them:
// without it the readiness checks report "no SSH address found" for every master converge
// did not touch.
func TestMasterHostsFromState(t *testing.T) {
	nodesState := map[string][]byte{
		"cluster-master-0": []byte(`{"outputs":{"master_ip_address_for_ssh":{"value":"10.12.0.174"}}}`),
		"cluster-master-1": []byte(`{"outputs":{"master_ip_address_for_ssh":{"value":"10.12.0.230"}}}`),
		// An immutable master answers no SSH and carries no address.
		"cluster-master-2": []byte(`{"outputs":{}}`),
		// A master whose state was never written: commander keeps it in its own cache.
		"cluster-master-3": nil,
	}

	require.Equal(t, []session.Host{
		{Host: "10.12.0.174", Name: "cluster-master-0"},
		{Host: "10.12.0.230", Name: "cluster-master-1"},
	}, MasterHostsFromState(nodesState))
}

// The session names a host after its own address, so it never matches a node name. The
// state must not be shadowed by it, while the cache — rewritten on every rebuild — must win.
func TestMergeMasterHostsPrefersTheCacheOverTheState(t *testing.T) {
	stateHosts := []session.Host{{Host: "10.12.1.33", Name: "cluster-master-2"}}
	sessionHosts := []session.Host{{Host: "10.12.0.174", Name: "10.12.0.174"}}
	cachedHosts := []session.Host{{Host: "10.12.9.9", Name: "cluster-master-2"}}

	require.Equal(t, []session.Host{
		{Host: "10.12.0.174", Name: "10.12.0.174"},
		{Host: "10.12.9.9", Name: "cluster-master-2"},
	}, MergeMasterHosts(stateHosts, sessionHosts, cachedHosts))
}
