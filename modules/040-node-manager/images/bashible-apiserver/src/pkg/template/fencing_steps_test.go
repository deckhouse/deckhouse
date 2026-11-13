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
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const sysctlTunerStepName = "041_configure_sysctl_tuner.sh.tpl"
const sysctlTunerStepPath = "../../../../../../../candi/bashible/common-steps/all/" + sysctlTunerStepName

// node-controller already hands bashible the normalized mode; Watchdog stays in the list as a safeguard.
func TestSysctlTunerKernelPanic(t *testing.T) {
	fencing := func(mode string) map[string]interface{} {
		return map[string]interface{}{"mode": mode}
	}
	cases := []struct {
		name  string
		group string
		spec  map[string]interface{}
		panic string
	}{
		{name: "Normal", group: "worker", spec: fencing("Normal"), panic: "0"},
		{name: "Slow", group: "worker", spec: fencing("Slow"), panic: "0"},
		{name: "Critical", group: "worker", spec: fencing("Critical"), panic: "0"},
		{name: "deprecated Watchdog", group: "worker", spec: fencing("Watchdog"), panic: "0"},
		{name: "Disabled", group: "worker", spec: fencing("Disabled"), panic: "10"},
		{name: "no spec.fencing", group: "worker", panic: "10"},
		{name: "master in Normal", group: "master", spec: fencing("Normal"), panic: "0"},
		{name: "master without fencing", group: "master", panic: "10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodeGroup := map[string]interface{}{"name": tc.group}
			if tc.spec != nil {
				nodeGroup["fencing"] = tc.spec
			}
			body := renderSysctlTunerStep(t, nodeGroup)

			tuner, step, found := strings.Cut(body, "\nchmod +x /opt/deckhouse/bin/sysctl-tuner\n")
			require.True(t, found)
			require.Contains(t, tuner, "\nsysctl -w kernel.panic="+tc.panic+"\n")
			require.Equal(t, 1, strings.Count(tuner, "sysctl -w kernel.panic="), "one kernel.panic rule for every node")
			// The step applies the value at once, so the fencing agent does not wait for the timer.
			require.Contains(t, step, "\nsysctl -w kernel.panic="+tc.panic+"\n")
			// Step 063 runs the tuner before kubelet; the timer restores a manual change.
			require.Contains(t, body, "OnUnitActiveSec=10min")
		})
	}
}

// The steps differ only between fencing on and off, so a change of the mode must not update the nodes.
func TestChecksumDependsOnlyOnFencingEnabled(t *testing.T) {
	step, err := os.ReadFile(sysctlTunerStepPath)
	require.NoError(t, err)
	root := t.TempDir()
	writeStepTemplate(t, root, "bashible/common-steps/all/"+sysctlTunerStepName, string(step))

	checksum := func(mode string) string {
		group := nodeGroup{"name": "worker", "nodeType": "Static"}
		if mode != "" {
			group["fencing"] = map[string]interface{}{"mode": mode}
		}
		builder := NewContextBuilder(context.Background(), NewStepsStorage(context.Background(), root, nil))
		builder.SetInputData(inputData{NodeGroups: []nodeGroup{group}})
		_, checksums, errs := builder.Build()
		require.Empty(t, errs)
		require.NotEmpty(t, checksums["worker"])
		return string(checksums["worker"])
	}

	normal := checksum("Normal")
	require.Equal(t, normal, checksum("Slow"))
	require.Equal(t, normal, checksum("Critical"))

	disabled := checksum("Disabled")
	require.NotEqual(t, normal, disabled)
	require.Equal(t, disabled, checksum(""), "Disabled and no spec.fencing give the same nodes")
}

func TestSysctlTunerReceiveBuffer(t *testing.T) {
	for _, group := range []string{"worker", "master"} {
		t.Run(group, func(t *testing.T) {
			body := renderSysctlTunerStep(t, map[string]interface{}{"name": group})
			require.Contains(t, body, "\nsysctl -w net.core.rmem_max=16777216\n")
		})
	}
}

func renderSysctlTunerStep(t *testing.T, nodeGroup map[string]interface{}) string {
	t.Helper()
	content, err := os.ReadFile(sysctlTunerStepPath)
	require.NoError(t, err)
	rendered, err := RenderTemplate(sysctlTunerStepName, content, map[string]interface{}{
		"nodeGroup": nodeGroup,
		"deckhouse": map[string]interface{}{"edition": "FE"},
	})
	require.NoError(t, err)
	body := rendered.Content.String()
	command := exec.Command("bash", "-n")
	command.Stdin = strings.NewReader(body)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	return body
}
