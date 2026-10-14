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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTheStaticNodeCleanupRemovesTheAgent(t *testing.T) {
	chdirToRepositoryRoot(t)

	script := renderFile(t, "candi/bashible/cleanup_static_node.sh.tpl", bashibleContext("Normal", map[string]any{}))

	require.Contains(t, bashArray(t, script, "SERVICES_TO_REMOVE"), "nodelet.service")
	require.Contains(t, bashArray(t, script, "SYSTEMD_FILES"), "/etc/systemd/system/nodelet.service")
	require.Contains(t, bashArray(t, script, "PATHS_TO_REMOVE"), "/var/lib/nodelet")
	requireBashSyntax(t, script)
}
func bashArray(t *testing.T, script, name string) []string {
	t.Helper()
	_, rest, found := strings.Cut(script, "\n"+name+"=(\n")
	require.True(t, found, "no %s array", name)
	body, _, found := strings.Cut(rest, "\n)")
	require.True(t, found, "%s is not closed", name)

	var entries []string
	for _, line := range strings.Split(body, "\n") {
		entries = append(entries, strings.TrimSpace(line))
	}
	return entries
}
