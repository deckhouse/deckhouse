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

// The proxy ClusterConfiguration names, as the two steps that leave it on the node write it: step
// 003 into the runtime's and kubelet's systemd drop-ins, step 053 into the registry agent's static pod
// — the agent being the process that fetches from the upstream registry, since the runtime asks the
// agent on the loopback address.
//
// Both take the values from their own bashible context and not from bb-set-proxy. The context is what
// the configuration checksum is computed from; bashbooster, where bb-set-proxy lives, is rendered later
// by helm and is not part of the checksum, so a node sent through bashible by a proxy change used to
// write the old proxy and was never sent through again.
//
// The steps' shell is run here under bash, with the bashbooster functions they call stubbed, and what
// they write is read back.
package bashible_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

const proxyStep = "all/003_add_system_proxy.sh.tpl"

const (
	testProxy = "http://user:p%40ss@proxy.example:3128"
	// What bb-set-proxy exported for the same cluster, and what the steps must still produce, so that an
	// upgrade leaves every drop-in byte for byte as it was.
	wantNoProxy = "127.0.0.1,169.254.169.254,registry.d8-system.svc,cluster.local,10.111.0.0/16,10.222.0.0/16,10.12.0.0/16"
)

// contextProxy is the `proxy` node-controller assembles into the bashible context.
func contextProxy(httpProxy, httpsProxy string) map[string]any {
	proxy := map[string]any{"noProxy": []any{"127.0.0.1", "169.254.169.254", "cluster.local",
		"10.111.0.0/16", "10.222.0.0/16", "10.12.0.0/16"}}
	if httpProxy != "" {
		proxy["httpProxy"] = httpProxy
	}
	if httpsProxy != "" {
		proxy["httpsProxy"] = httpsProxy
	}
	return proxy
}

func renderWithProxy(t *testing.T, step string, proxy map[string]any) string {
	t.Helper()

	path := filepath.Join(candi, step)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "cannot read %s", path)

	ctx := nodeContext(t, agentRegistry())
	if proxy != nil {
		ctx["proxy"] = proxy
	}
	rendered, err := template.RenderTemplate(filepath.Base(step), content, ctx)
	require.NoError(t, err, "cannot render %s", step)
	return rendered.Content.String()
}

// bashbooster stand-ins. bb-set-proxy exports a DIFFERENT proxy on purpose: it plays the stale
// bashbooster of the race, and a step that still called it would write that one.
const bashboosterStubs = `
bb-set-proxy() { export HTTP_PROXY=http://stale.example:1 HTTPS_PROXY=http://stale.example:1 NO_PROXY=stale; }
bb-unset-proxy() { unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy; }
`

// agentPod runs step 053's proxy block and returns the static pod the node would be given.
func agentPod(t *testing.T, proxy map[string]any) *corev1.Pod {
	t.Helper()

	body := renderWithProxy(t, agentStep, proxy)

	const first, last = "proxy_env=\"\"\n", "bb-unset-proxy\n"
	start := strings.Index(body, first)
	require.GreaterOrEqual(t, start, 0, "the step no longer builds the agent's proxy environment")
	end := strings.Index(body[start:], last)
	require.GreaterOrEqual(t, end, 0, "the step builds a proxy environment and never unsets the proxy after it")
	block := body[start : start+end+len(last)]

	script := "set -euo pipefail\n" +
		// Inherited from bashible's environment, and not the cluster's: it must not reach the agent.
		"export HTTPS_PROXY=http://inherited.example:1\n" +
		bashboosterStubs + block +
		"[ -z \"${HTTPS_PROXY:-}\" ] || { echo 'the proxy leaked past the step' >&2; exit 1; }\n" +
		"printf '%s' \"$proxy_env\"\n"
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	require.NoError(t, err, "the step's proxy block does not run: %s", out)

	const placeholder = "\n${proxy_env}\n"
	require.Equal(t, 1, strings.Count(body, placeholder),
		"the manifest takes the proxy environment on a line of its own, once")
	return staticPod(t, strings.Replace(body, placeholder, "\n"+string(out)+"\n", 1))
}

func envOf(pod *corev1.Pod) map[string]string {
	env := map[string]string{}
	for _, variable := range pod.Spec.Containers[0].Env {
		env[variable.Name] = variable.Value
	}
	return env
}

func TestTheAgentRunsWithTheClusterProxy(t *testing.T) {
	t.Run("with a proxy configured the agent is given it, in both spellings", func(t *testing.T) {
		env := envOf(agentPod(t, contextProxy(testProxy, testProxy)))

		for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
			require.Equal(t, testProxy, env[name], "%s is what the agent's HTTP client reads", name)
		}
		require.Equal(t, wantNoProxy, env["NO_PROXY"])
		require.Equal(t, wantNoProxy, env["no_proxy"])

		_, hasNodeName := env["NODE_NAME"]
		require.True(t, hasNodeName, "the proxy is added to the agent's environment, not instead of it")
	})

	t.Run("an HTTPS-only proxy sets only what it names", func(t *testing.T) {
		env := envOf(agentPod(t, contextProxy("", testProxy)))

		require.Equal(t, testProxy, env["HTTPS_PROXY"])
		require.NotContains(t, env, "HTTP_PROXY")
	})

	t.Run("without a proxy the agent's environment is what it was", func(t *testing.T) {
		pod := agentPod(t, nil)

		require.Len(t, pod.Spec.Containers[0].Env, 1, "nothing inherited from bashible's own environment")
		require.Equal(t, "NODE_NAME", pod.Spec.Containers[0].Env[0].Name)
	})
}

// TestTheRuntimeDropInsFollowTheContext is the race itself: the drop-ins step 003 writes carry the
// proxy of the context the step was rendered from, whatever bashbooster says.
func TestTheRuntimeDropInsFollowTheContext(t *testing.T) {
	root := t.TempDir()
	body := renderWithProxy(t, proxyStep, contextProxy(testProxy, testProxy))
	body = strings.ReplaceAll(body, "/etc/systemd/system/", root+"/")

	script := "set -euo pipefail\n" + bashboosterStubs + `
bb-event-on() { :; }
bb-flag-set() { :; }
systemctl() { :; }
bb-sync-file() { cat >"$1"; }
# The step edits in place with GNU sed's -i; BSD sed wants a suffix there. Same edit, either sed.
sed() {
  if [ "$1" = -i ]; then command sed "$2" "$3" >"$3.new" && mv "$3.new" "$3"; else command sed "$@"; fi
}
` + body
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	require.NoError(t, err, "step 003 does not run: %s", out)

	for _, unit := range []string{"containerd-deckhouse.service.d", "kubelet.service.d"} {
		dropIn, err := os.ReadFile(filepath.Join(root, unit, "proxy-environment.conf"))
		require.NoError(t, err, "step 003 wrote no drop-in for %s", unit)
		content := string(dropIn)

		// systemd needs % doubled, and the step does that after writing.
		escaped := strings.ReplaceAll(testProxy, "%", "%%")
		require.Contains(t, content, `"HTTPS_PROXY=`+escaped+`"`, "%s carries the context's proxy", unit)
		require.Contains(t, content, `"HTTP_PROXY=`+escaped+`"`)
		require.Contains(t, content, `"NO_PROXY=`+wantNoProxy+`"`,
			"%s carries the NO_PROXY bb-set-proxy built, so an upgrade does not restart the runtime", unit)
		require.NotContains(t, content, "stale.example", "%s was written from bashbooster, not the context", unit)
	}
}
