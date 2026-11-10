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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg/hook"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	"github.com/stretchr/testify/require"
)

// The hook prints its config for any arguments: shell-operator only calls it
// with --config here.
const configMapHook = `#!/bin/sh
cat <<'EOF'
configVersion: v1
kubernetes:
- name: configmaps
  apiVersion: v1
  kind: ConfigMap
EOF
`

func TestReloadHooksStopsMonitorsOfReplacedHooks(t *testing.T) {
	hooksDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "configmaps.sh"), []byte(configMapHook), 0o755))

	ctx := t.Context()
	cluster := fake.NewFakeCluster(fake.ClusterVersionV135)
	kubeEvents := kubeeventsmanager.NewKubeEventsManager(ctx, cluster.Client, log.NewNop())
	t.Cleanup(kubeEvents.Stop)

	shOp := &shell_operator.ShellOperator{
		KubeEventsManager: kubeEvents,
		HookManager: hook.NewHookManager(&hook.ManagerConfig{
			WorkingDir:        hooksDir,
			TempDir:           t.TempDir(),
			KubeEventsManager: kubeEvents,
			Logger:            log.NewNop(),
		}),
	}

	monitorIDs := func() []string {
		var ids []string
		for _, name := range shOp.HookManager.GetHookNames() {
			for _, binding := range shOp.HookManager.GetHook(name).GetConfig().OnKubernetesEvents {
				ids = append(ids, binding.Monitor.Metadata.MonitorId)
			}
		}
		return ids
	}

	require.NoError(t, reloadHooks(ctx, shOp, log.NewNop()))
	replaced := monitorIDs()
	require.Len(t, replaced, 1)

	require.NoError(t, reloadHooks(ctx, shOp, log.NewNop()))
	current := monitorIDs()
	require.Len(t, current, 1)
	require.NotEqual(t, replaced, current, "Init must give the reloaded hook a new MonitorId")

	require.False(t, kubeEvents.HasMonitor(replaced[0]),
		"the monitor of the replaced hook must be stopped, otherwise every reload leaks a monitor")
	require.True(t, kubeEvents.HasMonitor(current[0]),
		"the monitor of the current hook must stay: its snapshots feed the webhooks")
}
