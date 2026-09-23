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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestHostnameRule is the rule bashible applies at step 005 — after eleven packages have been
// installed, and from inside the retry storm. The node's certificates are issued for this name,
// so a node that fails it has to be recreated.
func TestHostnameRule(t *testing.T) {
	tests := []struct {
		hostname string
		valid    bool
	}{
		{"master-0", true},
		{"node1.cluster.example.com", true},
		{"a", true},
		{strings.Repeat("a", 63), true},

		{strings.Repeat("a", 64), false},
		{"Master-0", false},
		{"master_0", false},
		{"-master-0", false},
		{"master-0-", false},
		{".master", false},
		{"master.", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			if got := nodeNamePattern.MatchString(tt.hostname); got != tt.valid {
				t.Errorf("nodeNamePattern.MatchString(%q) = %v, want %v", tt.hostname, got, tt.valid)
			}
		})
	}
}

// TestHostnameProblemNamesTheRuleThatWasBroken: restating all five rules tells the reader nothing
// about their hostname.
func TestHostnameProblemNamesTheRuleThatWasBroken(t *testing.T) {
	tests := []struct {
		hostname string
		want     string
	}{
		{strings.Repeat("a", 70), "is 70 characters, the limit is 63"},
		{"Master-0", "contains upper-case letters"},
		{"master_0", "contains an underscore"},
		{"-master", "starts with '-' or '.'"},
		{"master-", "ends with '-' or '.'"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			assert.Equal(t, tt.want, hostnameProblem(tt.hostname))
		})
	}
}

func TestParseKernelVersion(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		ok      bool
	}{
		{"5.15.0-89-generic", 5, 15, true},
		{"6.1.0-13-amd64", 6, 1, true},
		{"4.18.0-513.5.1.el8_9.x86_64", 4, 18, true},
		{"6.12.28+bpo-amd64", 6, 12, true},
		{"", 0, 0, false},
		{"linux", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, ok := parseKernelVersion(tt.version)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.major, major)
				assert.Equal(t, tt.minor, minor)
			}
		})
	}
}

func TestParseSystemdVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		version int
		ok      bool
	}{
		{name: "ubuntu", output: "systemd 249 (249.11-0ubuntu3.12)\n+PAM +AUDIT", version: 249, ok: true},
		{name: "debian", output: "systemd 252 (252.22-1~deb12u1)", version: 252, ok: true},
		{name: "too old", output: "systemd 219 (219)", version: 219, ok: true},
		{name: "nothing to read", output: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, ok := parseSystemdVersion(tt.output)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.version, version)
			}
		})
	}
}

func TestParseDfOutput(t *testing.T) {
	// The POSIX format, with the device name on its own line as df wraps it when it is long.
	output := "Filesystem     1024-blocks     Used Available Capacity Mounted on\n" +
		"/dev/mapper/ubuntu--vg-ubuntu--lv  102687672 21458392  76965624      22% /\n"

	total, free, ok := parseDfOutput(output)
	require.True(t, ok)
	assert.Equal(t, 102687672, total)
	assert.Equal(t, 76965624, free)

	if _, _, ok := parseDfOutput("df: /var/lib: No such file or directory"); ok {
		t.Error("output that is not a df table must not parse")
	}
}

// TestCRIRequirementsOnlyApplyToContainerdV2: the default runtime has none of these requirements,
// and a check that asserts them anyway would refuse nodes that are perfectly fine.
func TestCRIRequirementsOnlyApplyToContainerdV2(t *testing.T) {
	for _, cri := range []string{"", "Containerd", "NotCRI"} {
		t.Run(cri, func(t *testing.T) {
			clusterConfig := map[string]json.RawMessage{}
			if cri != "" {
				encoded, err := json.Marshal(cri)
				require.NoError(t, err)
				clusterConfig["defaultCRI"] = encoded
			}

			check := NodeCRIRequirementsCheck{
				MetaConfig: &config.MetaConfig{ClusterConfig: clusterConfig},
				// Never reached: the runtime is decided from the configuration alone.
				NodeInterface: FixedNodeInterface(nil),
			}

			_, err := check.Run(t.Context())
			assert.ErrorIs(t, err, preflight.ErrNotApplicable)
		})
	}
}

// TestNodeLeftovers covers what bashible refuses at step 000, from inside the retry storm — and
// the case it does not get to at all: a node carrying a previous Deckhouse install, where
// "Bashible has already run! Skipping" sends the run on to fail later with a UUID mismatch.
func TestNodeLeftovers(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			// The default answer of the fake is "not on PATH", which is a clean node.
			name: "a clean node",
			node: newFakeNode(),
		},
		{
			// /opt/deckhouse/bin/containerd is the one Deckhouse installs: finding it is a node
			// being re-bootstrapped, not a pre-provisioned runtime.
			name: "Deckhouse's own containerd",
			node: newFakeNode().on("command -v containerd").prints(deckhouseContainerdPath),
		},
		{
			name:    "a pre-provisioned containerd",
			node:    newFakeNode().on("command -v containerd").prints("/usr/bin/containerd"),
			wantErr: "containerd at /usr/bin/containerd",
		},
		{
			name:    "docker",
			node:    newFakeNode().on("command -v dockerd").prints("/usr/bin/dockerd"),
			wantErr: "dockerd at /usr/bin/dockerd",
		},
		{
			name:    "a kubelet from another cluster",
			node:    newFakeNode().on("command -v kubelet").prints("/usr/bin/kubelet"),
			wantErr: "kubelet at /usr/bin/kubelet",
		},
		{
			name:    "a previous Deckhouse bootstrap",
			node:    newFakeNode().on("test -e /var/lib/bashible/bashible.sh").succeeds(),
			wantErr: "/var/lib/bashible, left by a previous Deckhouse bootstrap",
		},
		{
			// All of them at once: the node is cleaned once, not once per run.
			name: "several at once",
			node: newFakeNode().
				on("command -v containerd").prints("/usr/bin/containerd").
				on("command -v kubelet").prints("/usr/bin/kubelet"),
			wantErr: "kubelet at /usr/bin/kubelet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeLeftoversCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, "carries no container runtime")
		})
	}
}

// TestNodeHostnameRun covers the whole check, not just the pattern: the name is read off the node
// and the failure names the rule it broke.
func TestNodeHostnameRun(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "a name Kubernetes accepts",
			node: newFakeNode().on("hostname").prints("master-0\n"),
		},
		{
			name:    "upper case",
			node:    newFakeNode().on("hostname").prints("Master-0\n"),
			wantErr: "contains upper-case letters",
		},
		{
			name:    "an underscore",
			node:    newFakeNode().on("hostname").prints("master_0\n"),
			wantErr: "contains an underscore",
		},
		{
			name:    "nothing at all",
			node:    newFakeNode().on("hostname").prints("\n"),
			wantErr: "`hostname` printed nothing",
		},
		{
			name:    "hostname could not be run",
			node:    newFakeNode().on("hostname").fails(errors.New("session closed")),
			wantErr: "cannot read the hostname on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeHostnameCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, `is named "master-0"`)
		})
	}
}

// TestNodeHostnameFailureIsPermanent: the node's certificates are issued for this name, so a
// second attempt at the same check cannot find a different answer.
func TestNodeHostnameFailureIsPermanent(t *testing.T) {
	check := NodeHostnameCheck{
		NodeInterface: FixedNodeInterface(newFakeNode().on("hostname").prints("Master_0")),
	}

	_, err := check.Run(t.Context())
	require.Error(t, err)

	var permanent *backoff.PermanentError
	require.ErrorAs(t, err, &permanent, "a name that has to be changed is not worth retrying")
}
