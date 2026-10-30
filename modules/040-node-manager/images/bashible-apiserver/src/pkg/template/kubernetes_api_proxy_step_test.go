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

package template

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	apiProxyStepName = "051_pull_and_configure_kubernetes_api_proxy.sh.tpl"
	apiProxyStepPath = "../../../../../../../candi/bashible/common-steps/all/" + apiProxyStepName
	proxyConfigPath  = "../../../../kubernetes-api-proxy/src/internal/config/config.go"
)

// The proxy gets no probe flags: it uses the defaults of the binary, the same on every node.
var apiProxyArgs = []string{
	"--listen-address=127.0.0.1",
	"--listen-port=6445",
	"--health-listen=127.0.0.1:6480",
	"--log-level=info",
	"--as-static-pod=true",
	"--fallback-file=/var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json",
}

// The manifest of kubernetes-api-proxy has the same args for every CRI and every NodeGroup.
func TestKubernetesAPIProxyManifestArgs(t *testing.T) {
	groups := []map[string]interface{}{
		{"name": "worker"},
		{"name": "master"},
	}

	for _, cri := range []string{"Containerd", "ContainerdV2", "NotManaged"} {
		t.Run(cri, func(t *testing.T) {
			for _, group := range groups {
				require.Equal(t, apiProxyArgs, manifestArgs(t, renderAPIProxyStep(t, cri, group)), "NodeGroup %v", group)
			}
		})
	}
}

// The health endpoints of the proxy are on the address that the manifest gives to the proxy,
// and the default of the binary is the same address.
func TestKubernetesAPIProxyHealthListen(t *testing.T) {
	body := renderAPIProxyStep(t, "Containerd", map[string]interface{}{"name": "worker"})
	require.Contains(t, manifestArgs(t, body), "--health-listen=127.0.0.1:6480")
	require.Regexp(t, `(?m)^ +- name: health\n +containerPort: 6480\n`, body)

	require.Equal(t, "127.0.0.1:6480", sourceMatch(t, proxyConfigPath, `getenvDefault\("HEALTH_LISTEN", "([^"]+)"\)`),
		"default --health-listen of kubernetes-api-proxy")
}

// sourceMatch returns the first group of the first match of expr in the file.
func sourceMatch(t *testing.T, path, expr string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	m := regexp.MustCompile(expr).FindSubmatch(data)
	require.NotNil(t, m, "%s has no match for %s", path, expr)

	return string(m[1])
}

// manifestArgs returns the container args of the static pod manifest.
func manifestArgs(t *testing.T, body string) []string {
	t.Helper()

	_, rest, found := strings.Cut(body, "\n      args:\n")
	require.True(t, found, "no args in the manifest")

	var args []string

	arg := regexp.MustCompile(`^ +- "(.*)"$`)
	for _, line := range strings.Split(rest, "\n") {
		m := arg.FindStringSubmatch(line)
		if m == nil {
			break
		}

		args = append(args, m[1])
	}

	return args
}

func renderAPIProxyStep(t *testing.T, cri string, nodeGroup map[string]interface{}) string {
	t.Helper()

	content, err := os.ReadFile(apiProxyStepPath)
	require.NoError(t, err)

	rendered, err := RenderTemplate(apiProxyStepName, content, map[string]interface{}{
		"cri":       cri,
		"nodeGroup": nodeGroup,
		"registry":  map[string]interface{}{"imagesBase": "registry.example.com/deckhouse"},
		"images": map[string]interface{}{
			"nodeManager": map[string]interface{}{"kubernetesApiProxy": "sha256:0123456789abcdef"},
		},
	})
	require.NoError(t, err)

	body := rendered.Content.String()

	command := exec.Command("bash", "-n")
	command.Stdin = strings.NewReader(body)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)

	return body
}
