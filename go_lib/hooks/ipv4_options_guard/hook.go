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

package ipv4_options_guard

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	ModeAudit   = "Audit"
	ModeEnforce = "Enforce"

	// The decision is kept in the module's own ConfigMap: the hook passes it through values
	// and the template writes it there, so nothing is created outside the module.
	settingsConfigMap = "ipv4-options-guard"
	defaultModeKey    = "defaultMode"
)

// Agent DaemonSets of all CNI modules: any of them means the cluster existed before the guard.
var agentNamespaces = []string{"d8-cni-cilium", "d8-cni-flannel", "d8-cni-simple-bridge"}
var agentNames = []string{"agent", "flannel", "simple-bridge"}

func applyDefaultModeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1core.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return "", fmt.Errorf("from unstructured: %w", err)
	}
	return cm.Data[defaultModeKey], nil
}

func applyAgentFilter(_ *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return true, nil
}

func validDefault(mode string) bool {
	return mode == ModeAudit || mode == ModeEnforce
}

// RegisterHook sets <module>.internal.ipv4OptionsGuardMode: the mode from the module settings or,
// if it is not set, Enforce for a new cluster and Audit for a cluster that existed before.
func RegisterHook(moduleName, namespace string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "default_mode",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{namespace}},
				},
				NameSelector:                 &types.NameSelector{MatchNames: []string{settingsConfigMap}},
				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				FilterFunc:                   applyDefaultModeFilter,
			},
			{
				Name:       "agents",
				ApiVersion: "apps/v1",
				Kind:       "DaemonSet",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: agentNamespaces},
				},
				NameSelector:                 &types.NameSelector{MatchNames: agentNames},
				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				FilterFunc:                   applyAgentFilter,
			},
		},
	}, setMode(moduleName))
}

func setMode(moduleName string) func(_ context.Context, input *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		defaultPath := moduleName + ".internal.ipv4OptionsGuardDefaultMode"

		recorded, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "default_mode")
		if err != nil {
			return fmt.Errorf("unmarshal default_mode snapshot: %w", err)
		}
		d := decision{
			Pending:    input.Values.Get(defaultPath).String(),
			Existing:   len(input.Snapshots.Get("agents")) > 0 || input.Values.Get("global.clusterIsBootstrapped").Bool(),
			Configured: input.Values.Get(moduleName + ".ipv4OptionsGuard.mode").String(),
		}
		if len(recorded) > 0 {
			d.Recorded = recorded[0]
		}
		defaultMode, mode := d.modes()
		input.Values.Set(defaultPath, defaultMode)
		input.Values.Set(moduleName+".internal.ipv4OptionsGuardMode", mode)
		return nil
	}
}

type decision struct {
	// Recorded is defaultMode from the ConfigMap of the module.
	Recorded string
	// Pending is the default mode decided by an earlier run, but not rendered into the ConfigMap yet.
	Pending string
	// Existing: a CNI agent is running (covers clusters without worker nodes) or the cluster is
	// bootstrapped (covers switching the CNI after the agent of the previous one is removed).
	Existing bool
	// Configured is the mode from the module settings.
	Configured string
}

// modes returns the default mode and the effective one. A lost decision on a running cluster
// falls back to Audit, never to Enforce.
func (d decision) modes() (string, string) {
	defaultMode := d.Recorded
	if !validDefault(defaultMode) {
		defaultMode = d.Pending
	}
	if !validDefault(defaultMode) {
		defaultMode = ModeEnforce
		if d.Existing {
			defaultMode = ModeAudit
		}
	}
	if d.Configured != "" {
		return defaultMode, d.Configured
	}
	return defaultMode, defaultMode
}
