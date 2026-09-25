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

package checks

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestRegistryFromMasterClassifiesTheCause: the node not reaching the registry has four causes
// that need four different things done about them, and they used to share one sentence about
// egress and security groups.
//
// The name not resolving is the one that matters most. It is what a node with the wrong resolver
// does, and it reaches the operator as "etcd not running after 200s" from inside the bashible
// retry storm — because crictl could not resolve the registry and never pulled the image. Nothing
// in that message mentions DNS, and nothing before it asked.
func TestRegistryFromMasterClassifiesTheCause(t *testing.T) {
	const (
		hostname = "registry.company.my"
		address  = "registry.company.my:5000"
	)

	tests := []struct {
		name string
		// status is what curl exited with.
		status       int
		wantObserved string
		wantFix      string
		wantNotInFix string
	}{
		{
			name:         "the name does not resolve",
			status:       6,
			wantObserved: "cannot resolve registry.company.my",
			wantFix:      "/etc/resolv.conf",
			// Sending the reader to the security groups is what this check used to do, and it
			// is the wrong place entirely.
			wantNotInFix: "security group",
		},
		{
			name:         "nothing is listening",
			status:       7,
			wantObserved: "could not connect to registry.company.my:5000",
			wantFix:      "egress",
		},
		{
			name:         "the connection times out",
			status:       28,
			wantObserved: "timed out",
			wantFix:      "egress",
		},
		{
			name:         "the certificate is rejected",
			status:       60,
			wantObserved: "rejected the registry certificate",
			wantFix:      "CA",
		},
		{
			name:         "a status with no meaning of its own",
			status:       2,
			wantObserved: "could not reach the registry",
			wantFix:      "egress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observed, fix := curlClassify(&fakeSSHExitError{status: tt.status}, hostname, address, "Direct")

			assert.Contains(t, observed, tt.wantObserved)
			assert.Contains(t, fix, tt.wantFix)
			if tt.wantNotInFix != "" {
				assert.NotContains(t, fix, tt.wantNotInFix)
			}
		})
	}

	t.Run("an error with no exit status at all", func(t *testing.T) {
		// The command never ran — a dropped session, say. There is nothing to classify.
		observed, fix := curlClassify(errors.New("session closed"), hostname, address, "Direct")

		assert.Contains(t, observed, "could not reach the registry")
		assert.Contains(t, fix, "egress")
	})

	t.Run("wget separates only the certificate", func(t *testing.T) {
		// It collapses every network failure into one status, so claiming more would be a guess.
		observed, _ := wgetClassify(&fakeSSHExitError{status: 5}, hostname, address, "Direct")
		assert.Contains(t, observed, "certificate")

		observed, fix := wgetClassify(&fakeSSHExitError{status: 4}, hostname, address, "Direct")
		assert.Contains(t, observed, "could not reach the registry")
		assert.Contains(t, fix, "egress")
	})
}

// TestRegistryFromMasterProbeWithAPrivateCA: bashible installs the registry CA on the node, and
// this check runs before it. Verifying the certificate here would refuse a node that is perfectly
// able to pull once configured — and the certificate is registry-reachable's question anyway, from
// the installer, which does have the CA.
func TestRegistryFromMasterProbeWithAPrivateCA(t *testing.T) {
	node := newFakeNode().on("command -v curl").succeeds()

	probe, err := nodeRegistryProbe(t.Context(), node, true)
	assert.NoError(t, err)
	assert.Contains(t, probe.args("https://registry.company.my/v2/"), "-k")

	verifying, err := nodeRegistryProbe(t.Context(), node, false)
	assert.NoError(t, err)
	assert.NotContains(t, verifying.args("https://registry.company.my/v2/"), "-k")
}

// This probe goes direct, so it has nothing to say about a cluster whose nodes reach the registry
// through a proxy — and asking anyway demanded egress the cluster was never going to have. Bought
// live on a static cluster behind 192.168.199.254:8888: HTTP 000 here while
// registry-access-through-proxy, which asks the same question the right way, passed.
func TestRegistryFromMasterStepsAsideForTheProxy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("a proxy the registry is not exempt from", func(t *testing.T) {
		check := RegistryFromMasterCheck{MetaConfig: metaConfigWithProxy(t, server, "http://192.168.199.254:8888", nil)}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "registry-access-through-proxy")
	})

	// noProxy puts the registry back on the direct path, which is this check's subject again.
	t.Run("a registry exempted by noProxy", func(t *testing.T) {
		check := RegistryFromMasterCheck{MetaConfig: metaConfigWithProxy(t, server, "http://192.168.199.254:8888", []string{proxiedRegistryHost})}

		_, err := check.Run(t.Context())

		// It goes on to ask the node, which this test gives it no way to reach — what matters is
		// that it did not step aside for the proxy.
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "registry-access-through-proxy")
	})
}
