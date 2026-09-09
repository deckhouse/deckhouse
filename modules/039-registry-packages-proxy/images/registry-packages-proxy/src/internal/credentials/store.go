/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package credentials

import (
	"encoding/base64"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

const (
	// storeAccessSecret holds how to read the cluster's own store: its authority and a read-only
	// account. It is rendered only where the module actually runs a store, so its presence is itself
	// the answer to "is the in-cluster registry a store of ours".
	storeAccessSecret = "registry-storage-access"
)

// storeAuthority is how this proxy reaches the cluster's own store while the node has no agent.
//
// It exists because of a circle that is real and was measured rather than reasoned about. The node
// agent is installed from a package fetched THROUGH this proxy — bashible step 034, `rpp-get
// [registry-agent]` — so on a node whose agent does not exist yet, this process is what has to reach a
// registry, and the agent cannot be the way it does so. Measured on `ly-cache` with no fallback at all:
//
//	[rpp-get] [registry-agent] attempt 8 failed: access to https://10.110.0.9:4219/package?digest=...
//	proxy: get package "sha256:1255..." : Get "https://registry.d8-system.svc:5001/..."
//	/etc/kubernetes/registry-agent/pki/ca.crt: No such file or directory
//
// The bootstrap then times out waiting for a worker that can never join, because joining needs
// `rpp-get` and `rpp-get` needs this.
//
// What it is NOT: a way to reach an upstream with credentials out of a secret. That is the shape the
// owner ruled out, and this is not it — the address here is the cluster's own store, the credentials
// are the store's own read-only account, and nothing leaves the cluster. Reaching an UPSTREAM stays the
// agent's business alone.
//
// Why the installer's `deckhouse-registry` secret cannot serve instead: after the handover that address
// is served by the store, whose authority is `CN = registry-storage-ca` while the secret still describes
// the installer's bootstrap PKI (`CN = registry-ca`) — measured as `x509: certificate signed by unknown
// authority` on every fetch, which is the failure this whole path was first written for.
func storeAuthority(data map[string][]byte, repository string) *registry.ClientConfig {
	authority := string(data["ca.crt"])
	username := string(data["username"])
	password := string(data["password"])

	// All three or nothing. A config with an authority and no account authenticates nowhere, and one
	// with an account and no authority fails the handshake — either way it would replace a working
	// answer with a broken one, which is worse than having no answer at all.
	if authority == "" || username == "" || password == "" {
		return nil
	}

	return &registry.ClientConfig{
		Repository: repository,
		Scheme:     "https",
		CA:         authority,
		Auth: base64.StdEncoding.EncodeToString(
			[]byte(fmt.Sprintf("%s:%s", username, password)),
		),
	}
}
