/*
Copyright 2024 Flant JSC

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
	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

const (
	providerConfigKey = "cloud-provider-cluster-configuration.yaml"
	secretNs          = "kube-system"
	secretName        = "d8-provider-cluster-configuration"
)

type HookParams struct {
	OldRootDiskSize                       int64
	MasterRootDiskSizeFieldPath           []string
	GenericNodeGroupRootDiskSizeFieldPath []string
}

func RegisterHook(params *HookParams) bool {
	hookHandler := func(input *go_hook.HookInput) error {
		return migrateDiskGBHandler(input, params)
	}

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "provider_configuration",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{secretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{secretNs},
					},
				},
				ExecuteHookOnEvents: ptr.To(false),
				FilterFunc:          providerConfigurationSecretFilter,
			},
			{
				Name:       "install_version",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"install-data"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-system"},
					},
				},
				ExecuteHookOnEvents: ptr.To(false),
				FilterFunc:          installDataCMFilter,
			},
		},
	}, hookHandler)
}

func providerConfigurationSecretFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(unstructured, &secret)
	if err != nil {
		return nil, err
	}

	return &secret, nil
}

func installDataCMFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	if version, ok := cm.Data["version"]; ok {
		return version, nil
	}

	return "", nil
}

func migrateDiskGBHandler(input *go_hook.HookInput, hookParams *HookParams) error {
	providerConfigSecretSnap := input.Snapshots["provider_configuration"]
	if len(providerConfigSecretSnap) == 0 {
		return nil
	}

	needMigration, err := needMigrateForDeckhouseInstallVersion(input.Snapshots)
	if err != nil {
		return err
	}

	if !needMigration {
		input.Logger.Info("Skipping migration of root disk volume")
		return nil
	}

	providerConfigSecret := providerConfigSecretSnap[0].(*corev1.Secret)

	backupSecret := providerConfigSecret.DeepCopy()

	var rawConfig map[string]interface{}
	err = yaml.Unmarshal(providerConfigSecret.Data[providerConfigKey], &rawConfig)
	if err != nil {
		return err
	}

	needMigratieMasters, err := needMigrateMasterInstanceClass(
		rawConfig,
		hookParams.OldRootDiskSize,
		hookParams.MasterRootDiskSizeFieldPath,
	)
	if err != nil {
		return err
	}

	needMigrateNGs, err := needMigrateNodeGroupsInstanceClass(
		rawConfig,
		hookParams.OldRootDiskSize,
		hookParams.GenericNodeGroupRootDiskSizeFieldPath,
	)
	if err != nil {
		return err
	}

	if !needMigratieMasters && !needMigrateNGs {
		input.Logger.Info("Skipping migration diskSizeGB because migration already done or diskSizeGB already set")
		return nil
	}

	backupSecret.Name += `-bkp-disk-gb`
	backupSecret.ResourceVersion = ""
	input.PatchCollector.Create(backupSecret, object_patch.IgnoreIfExists())

	data, err := yaml.Marshal(rawConfig)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			providerConfigKey: data,
		},
	}

	input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", secretName)

	return err
}

func hasRootDiskSizeProperty(rawConfig map[string]interface{}, fields []string) (bool, error) {
	_, found, err := unstructured.NestedFieldNoCopy(rawConfig, fields...)
	if err != nil {
		return false, err
	}

	return found, nil
}

func needMigrateNodeGroupsInstanceClass(rawConfig map[string]interface{}, oldSize int64, sizeFieldPath []string) (bool, error) {
	nodeGroups, found, err := unstructured.NestedSlice(rawConfig, "nodeGroups")
	if err != nil {
		return false, err
	}

	if !found {
		// we can do not have nodegroups. skip
		return false, nil
	}

	needMigrate := false

	resultNgs := make([]interface{}, 0, len(nodeGroups))

	for _, rawNG := range nodeGroups {
		ng := rawNG.(map[string]interface{})
		found, err := hasRootDiskSizeProperty(ng, sizeFieldPath)
		if err != nil {
			return false, err
		}

		if found {
			resultNgs = append(resultNgs, rawNG)
			continue
		}

		err = unstructured.SetNestedField(ng, oldSize, sizeFieldPath...)
		if err != nil {
			return false, err
		}

		needMigrate = true
		resultNgs = append(resultNgs, ng)
	}

	if !needMigrate {
		return false, nil
	}

	err = unstructured.SetNestedSlice(rawConfig, resultNgs, "nodeGroups")
	if err != nil {
		return false, err
	}

	return true, nil
}

func needMigrateMasterInstanceClass(rawConfig map[string]interface{}, oldSize int64, sizeFieldPath []string) (bool, error) {
	found, err := hasRootDiskSizeProperty(rawConfig, sizeFieldPath)
	if err != nil {
		return false, err
	}

	if found {
		return false, nil
	}

	err = unstructured.SetNestedField(rawConfig, oldSize, sizeFieldPath...)
	if err != nil {
		return false, err
	}

	return true, nil
}

// check install version. if version > 1.63 we do not need migration because right default was set
func needMigrateForDeckhouseInstallVersion(snaps go_hook.Snapshots) (bool, error) {
	is := snaps["install_version"]
	if len(is) == 0 {
		// install-data configmap available from 1.55
		// https://github.com/deckhouse/deckhouse/pull/6522
		// if cm not found we should try to migration
		return true, nil
	}

	versionStr := is[0].(string)
	// do not migrate for dev build
	if versionStr == "dev" {
		return false, nil
	}

	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return false, err
	}

	if version.Compare(semver.MustParse("1.63.0")) >= 0 {
		return false, nil
	}

	return true, nil
}
