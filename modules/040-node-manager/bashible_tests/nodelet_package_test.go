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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const nodeletPackage = "modules/007-registrypackages/images/nodelet/scripts"

func TestTheNodeletPackageNeverStartsTheAgent(t *testing.T) {
	chdirToRepositoryRoot(t)

	install := readFile(t, nodeletPackage+"/install")
	require.Contains(t, install, "systemctl enable nodelet.service")
	for _, starts := range []string{"--now", "systemctl start", "systemctl restart", "systemctl reload"} {
		require.NotContains(t, install, starts)
	}
	require.Contains(t, install, "mv -f /opt/deckhouse/bin/nodelet.new /opt/deckhouse/bin/nodelet")

	unit := readFile(t, nodeletPackage+"/nodelet.service")
	require.Contains(t, unit, "ExecStart=/opt/deckhouse/bin/nodelet -config /var/lib/nodelet/nodeconfig.yaml")
	require.Contains(t, unit, "Restart=always")

	require.Contains(t, readFile(t, nodeletPackage+"/uninstall"), "systemctl disable --now nodelet.service")

	for _, script := range []string{"install", "uninstall"} {
		requireBashSyntax(t, readFile(t, nodeletPackage+"/"+script))
	}
}
func chdirToRepositoryRoot(t *testing.T) {
	t.Helper()
	t.Chdir("../../..")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
func requireBashSyntax(t *testing.T, script string) {
	t.Helper()
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
}
