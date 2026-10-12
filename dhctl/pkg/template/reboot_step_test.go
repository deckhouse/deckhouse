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

package template

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const rebootStepStubs = `
bb-flag?() { return 0; }
bb-flag-unset() { :; }
bb-log-info() { :; }
bb-deckhouse-get-disruptive-update-approval() { :; }
bb-label-node-bashible-first-run-finished() { :; }
shutdown() { echo "shutdown $*"; }
`

func TestRebootStepOnFirstRunSkipsRebootOnlyOnClusterBootstrap(t *testing.T) {
	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/common-steps/all/099_reboot.sh.tpl")
	require.NoError(t, err)

	wantRebootByRunType := map[string]bool{
		"ClusterBootstrap": false,
		"Normal":           true,
	}

	for runType, wantReboot := range wantRebootByRunType {
		t.Run(runType, func(t *testing.T) {
			rendered, err := RenderTemplate("099_reboot.sh.tpl", tplContent, map[string]any{"runType": runType})
			require.NoError(t, err)

			dir := t.TempDir()
			cmd := exec.Command("bash", "-c", rebootStepStubs+rendered.Content.String())
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"FIRST_BASHIBLE_RUN=yes",
				"BASHIBLE_INITIALIZED_FILE="+filepath.Join(dir, "bashible-fully-initialized"),
			)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))

			require.Equal(t, wantReboot, strings.Contains(string(out), "shutdown -r"), string(out))
		})
	}
}
