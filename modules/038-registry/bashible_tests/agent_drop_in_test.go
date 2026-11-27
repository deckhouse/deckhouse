/*
Copyright 2026 Flant JSC

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

package bashible_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const agentHeader = "# Managed by the Deckhouse registry agent. Do not edit."

func agentDropInSnippet(t *testing.T, dir string) string {
	t.Helper()

	body := render(t, "all/053_configure_registry_agent.sh.tpl", legacyRegistry())
	start := strings.Index(body, "# Only a drop-in the agent wrote")
	require.NotEqual(t, -1, start, "step 053 no longer removes the agent's drop-in")
	end := strings.Index(body[start:], "\nfi\n")
	require.NotEqual(t, -1, end)
	snippet := body[start : start+end+len("\nfi\n")]

	return strings.NewReplacer(
		"/etc/containerd/registry.d", dir,
		"/opt/deckhouse/bin/jq", "jq",
		"seq 1 60", "seq 1 3",
		"sleep 1", "sleep 0",
	).Replace(snippet)
}

// isolatedPath is a PATH of bin and the tools the snippet runs, so that a crictl on the host is not found.
func isolatedPath(t *testing.T, bin string) string {
	t.Helper()

	tools := t.TempDir()
	for _, tool := range []string{"grep", "seq", "sleep", "rm", "mv", "jq", "dirname", "touch", "cat", "mkdir"} {
		path, err := exec.LookPath(tool)
		require.NoError(t, err, tool)
		require.NoError(t, os.Symlink(path, filepath.Join(tools, tool)))
	}
	return bin + string(os.PathListSeparator) + tools
}

func TestTheAgentDropInGoesOnlyOnceTheAgentIsSeenStopped(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed")
	}

	// The fake crictl answers for the agent only when asked for kube-system's static pod, and records
	// what it was asked to stop. "$DROPIN" is the agent's drop-in directory.
	const (
		running     = `[[ "$*" == *io.kubernetes.pod.namespace=kube-system* ]] && echo 0123abcd`
		stopsOnStop = `stopped="$(dirname "$0")/stopped"; case "$1" in ps) [[ -f "$stopped" ]] || { ` + running + `; } ;; stop) echo "$*" >> "$(dirname "$0")/stops"; [[ "$*" == *0123abcd* ]] && touch "$stopped" ;; esac`
	)

	type dropInCase struct {
		name      string
		header    string // the drop-in at the start; empty: none
		crictl    string // the fake's body; empty: no crictl
		removed   bool
		stepFail  bool
		noForeign bool // nothing outside kube-system was stopped
	}
	cases := []dropInCase{
		{name: "the agent has stopped", header: agentHeader, crictl: "exit 0", removed: true},
		{name: "the agent does not stop even when stopped", header: agentHeader, crictl: running, stepFail: true},
		// The kubelet is not running, so nothing stops the agent but the step; it must, or the step
		// fails on every run and the kubelet, started by a later step, never is.
		{name: "the kubelet does not stop it, the step does", header: agentHeader, crictl: stopsOnStop, removed: true},
		{name: "crictl fails", header: agentHeader, crictl: "exit 1", stepFail: true},
		{name: "an administrator's drop-in", header: "# written by hand", crictl: "exit 0"},
		// Step 030 removed the drop-in while the agent still ran, and the agent wrote it back before it
		// stopped: waited for all the same, and removed after.
		{
			name: "no drop-in at the start, the agent writes it before it stops", crictl: `case "$1" in stop) mkdir -p "$DROPIN"; echo "` + agentHeader + `" > "$DROPIN/hosts.toml" ;; esac; ` + stopsOnStop,
			removed: true,
		},
		// A container of the same name in someone else's pod is neither waited for nor stopped.
		{
			name: "a same-named container in another pod", header: agentHeader,
			crictl:  `case "$1" in ps) [[ "$*" == *io.kubernetes.pod.namespace=kube-system* ]] || echo f0re1gn ;; stop) echo "$*" >> "$(dirname "$0")/stops" ;; esac`,
			removed: true, noForeign: true,
		},
		{name: "no crictl, the agent's drop-in", header: agentHeader, stepFail: true},
		{name: "no crictl, an administrator's drop-in", header: "# written by hand"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := t.TempDir()
			dropIn := filepath.Join(dir, "_default")
			if tc.header != "" {
				require.NoError(t, os.MkdirAll(dropIn, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dropIn, "hosts.toml"), []byte(tc.header+"\n"), 0o600))
			}
			state := filepath.Join(dir, "deckhouse_hosts_state.json")
			require.NoError(t, os.WriteFile(state, []byte(`["_default","other"]`), 0o600))

			path := isolatedPath(t, bin)
			if tc.crictl != "" {
				require.NoError(t, os.WriteFile(filepath.Join(bin, "crictl"), []byte("#!/bin/bash\n"+tc.crictl+"\n"), 0o755))
			}

			script := agentDropInSnippet(t, dir)
			cmd := exec.CommandContext(t.Context(), "bash", "--noprofile", "--norc", "-eEo", "pipefail", "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+path, "DROPIN="+dropIn)
			out, err := cmd.CombinedOutput()

			if tc.stepFail {
				require.Error(t, err, "the step must fail while the agent may restore its drop-in: %s", out)
			} else {
				require.NoError(t, err, "%s", out)
			}
			_, statErr := os.Stat(dropIn)
			if tc.removed {
				require.True(t, os.IsNotExist(statErr), "drop-in removed: %s", out)
				got, err := os.ReadFile(state)
				require.NoError(t, err)
				require.JSONEq(t, `["other"]`, string(got))
			} else if tc.header != "" {
				require.NoError(t, statErr, "drop-in kept")
			}
			if tc.noForeign {
				stops, _ := os.ReadFile(filepath.Join(bin, "stops"))
				require.NotContains(t, string(stops), "f0re1gn")
			}
		})
	}
}

// Step 030 stops a running agent before it writes the directory the agent owns: a live agent removes what
// the step writes, the registries' authorities and mirrors, and restores its `_default`.
func TestStep030StopsTheAgentBeforeItWrites(t *testing.T) {
	body := render(t, "all/030_configure_containerd_registry.sh.tpl", legacyRegistry())
	start := strings.Index(body, "# A node leaving the agent stops it")
	require.NotEqual(t, -1, start, "step 030 no longer stops the agent before it writes")
	end := strings.Index(body[start:], "\nfi\n")
	require.NotEqual(t, -1, end)
	snippet := body[start : start+end+len("\nfi\n")]
	require.Less(t, start, strings.Index(body, "mkdir -p"), "stopped before anything is written")

	const running = `[[ "$*" == *io.kubernetes.pod.namespace=kube-system* ]] && echo 0123abcd`
	cases := []struct {
		name     string
		manifest bool
		crictl   string // empty: no crictl
		stepFail bool
		stops    bool // the step itself asked crictl to stop the agent
	}{
		{name: "the agent stops once its manifest is gone", manifest: true, crictl: "exit 0"},
		{name: "the agent never stops", manifest: true, crictl: running, stepFail: true},
		{
			name: "the kubelet does not stop it, the step does", manifest: true,
			crictl: `stopped="$(dirname "$0")/stopped"; case "$1" in ps) [[ -f "$stopped" ]] || { ` + running + `; } ;; stop) touch "$stopped" ;; esac`,
		},
		// An earlier attempt of the step removed the manifest and failed; the kubelet is down, so the agent
		// still runs: found by crictl alone, and stopped before anything is written beside it.
		{
			name:   "no manifest, the agent still running",
			crictl: `stopped="$(dirname "$0")/stopped"; case "$1" in ps) [[ -f "$stopped" ]] || { ` + running + `; } ;; stop) echo "$*" >> "$(dirname "$0")/stops"; touch "$stopped" ;; esac`,
			stops:  true,
		},
		{name: "crictl fails with the manifest there", manifest: true, crictl: "exit 1", stepFail: true},
		{name: "a node that never had the agent", crictl: "exit 0"},
		{name: "no crictl to stop it", manifest: true, stepFail: true},
		{name: "no crictl and no agent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifests := t.TempDir()
			bin := t.TempDir()
			manifest := filepath.Join(manifests, "registry-agent.yaml")
			if tc.manifest {
				require.NoError(t, os.WriteFile(manifest, []byte("kind: Pod\n"), 0o600))
			}
			if tc.crictl != "" {
				require.NoError(t, os.WriteFile(filepath.Join(bin, "crictl"), []byte("#!/bin/bash\n"+tc.crictl+"\n"), 0o755))
			}

			script := strings.NewReplacer(
				"/etc/kubernetes/manifests", manifests,
				"seq 1 60", "seq 1 3",
				"sleep 1", "sleep 0",
			).Replace(snippet)
			cmd := exec.CommandContext(t.Context(), "bash", "--noprofile", "--norc", "-eEo", "pipefail", "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+isolatedPath(t, bin))
			out, err := cmd.CombinedOutput()

			if tc.stepFail {
				require.Error(t, err, "%s", out)
			} else {
				require.NoError(t, err, "%s", out)
			}
			_, statErr := os.Stat(manifest)
			require.True(t, os.IsNotExist(statErr), "the agent's manifest is gone either way")
			if tc.stops {
				stops, err := os.ReadFile(filepath.Join(bin, "stops"))
				require.NoError(t, err, "the step never stopped the agent: %s", out)
				require.Contains(t, string(stops), "0123abcd")
			}
		})
	}
}
