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

package hooks

import (
	"context"
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

// The D8CloudProviderDVPMigrationPending alert must not fire in clusters managed by
// Deckhouse Commander until Commander learns to apply the new-format migration
// resources: the admin cannot act on the alert in that case. Commander management is
// detected by the kube-system/d8-commander-uuid ConfigMap (created by dhctl in commander
// mode). Whether Commander already supports the new format is read from the
// d8-commander-agent/commander-info ConfigMap: flags.cloudProviderNoPCCInputFormatSupported
// ("1" means supported). The migration marker ConfigMap alone cannot express this, and the
// support flag lives inside a JSON blob (data.json) that PromQL cannot read, so this hook
// computes a single metric that the alert consumes directly.
const (
	migrationPendingMetricName  = "d8_cloud_provider_dvp_migration_pending"
	migrationPendingMetricGroup = "D8CloudProviderDVPMigration"

	commanderUUIDNamespace     = "kube-system"
	commanderUUIDConfigMapName = "d8-commander-uuid"

	commanderInfoNamespace     = "d8-commander-agent"
	commanderInfoConfigMapName = "commander-info"
	commanderInfoDataKey       = "data.json"
	commanderPCCSupportFlag    = "cloudProviderNoPCCInputFormatSupported"
)

// commanderInfoResult carries the parsed support flag from the commander-info ConfigMap.
type commanderInfoResult struct {
	Supported bool `json:"supported"`
}

// commanderInfoData mirrors the relevant part of commander-info data.json. Flag values are
// strings ("1"/"0"), not booleans.
type commanderInfoData struct {
	Flags map[string]string `json:"flags"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_migration_marker",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpMigrationConfigMapName},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterMigrationMarkerConfigMap,
		},
		{
			Name:       "commander_uuid",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{commanderUUIDNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{commanderUUIDConfigMapName},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterCommanderUUIDConfigMap,
		},
		{
			Name:       "commander_info",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{commanderInfoNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{commanderInfoConfigMapName},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterCommanderInfoConfigMap,
		},
	},
}, handleMigrationPendingMetric)

// filterMigrationMarkerConfigMap reports the presence of the migration marker ConfigMap.
func filterMigrationMarkerConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj.GetName() != dvpMigrationConfigMapName {
		return nil, nil
	}
	return obj.GetName(), nil
}

// filterCommanderUUIDConfigMap reports the presence of the commander-uuid ConfigMap.
func filterCommanderUUIDConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj.GetName() != commanderUUIDConfigMapName {
		return nil, nil
	}
	return obj.GetName(), nil
}

// filterCommanderInfoConfigMap extracts the cloudProviderNoPCCInputFormatSupported flag from
// the commander-info ConfigMap. A missing/empty/invalid data.json or flag is treated as "not supported"
func filterCommanderInfoConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj.GetName() != commanderInfoConfigMapName {
		return nil, nil
	}

	cm := &corev1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, err
	}

	result := commanderInfoResult{Supported: false}

	raw, ok := cm.Data[commanderInfoDataKey]
	if !ok || raw == "" {
		return result, nil
	}

	var data commanderInfoData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		// fail-closed: unparsable data.json means we cannot confirm support.
		return result, nil
	}

	result.Supported = data.Flags[commanderPCCSupportFlag] == "1"
	return result, nil
}

func handleMigrationPendingMetric(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(migrationPendingMetricGroup)

	migrationPresent := len(input.Snapshots.Get("module_migration_marker")) > 0
	if !migrationPresent {
		return nil
	}

	commanderManaged := len(input.Snapshots.Get("commander_uuid")) > 0

	commanderNoPCCSupported := false
	if infoSnaps := input.Snapshots.Get("commander_info"); len(infoSnaps) > 0 {
		var info commanderInfoResult
		if err := infoSnaps[0].UnmarshalTo(&info); err != nil {
			return err
		}
		commanderNoPCCSupported = info.Supported
	}

	if !commanderManaged || commanderNoPCCSupported {
		input.MetricsCollector.Set(
			migrationPendingMetricName,
			1,
			nil,
			metrics.WithGroup(migrationPendingMetricGroup),
		)
	}

	return nil
}
