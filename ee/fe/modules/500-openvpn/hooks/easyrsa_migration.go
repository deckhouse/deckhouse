/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "easyrsa_migrated",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"easyrsa-migrated"},
			},
			FilterFunc: applyMigrationSecretFilter,
		},
		{
			Name:       "openvpn_pvc",
			ApiVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"certs-openvpn-0"},
			},
			FilterFunc: applyMigrationPVCFilter,
		},
		{
			Name:       "openvpn_sts",
			ApiVersion: "apps/v1",
			Kind:       "StatefulSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"openvpn"},
			},
			FilterFunc: applyMigrationSTSFilter,
		},
	},
}, migration)

func applyMigrationSTSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sts = &appsv1.StatefulSet{}
	err := sdk.FromUnstructured(obj, sts)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %s", err.Error())
	}

	return sts.Annotations["easyrsa-migrated"], nil
}

func applyMigrationPVCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pvc := &v1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %s", err.Error())
	}

	return *pvc.Spec.StorageClassName, nil
}

func applyMigrationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %s", err.Error())
	}

	return secret.Name, nil
}

func migration(input *go_hook.HookInput) error {
	// We stopped using the disk, so this option is no longer needed. To avoid validation errors, before removing storageClass from the spec, we need to remove it from the config in all existing installations.
	input.ConfigValues.Remove("openvpn.storageClass")

	migrated := false

	if len(input.Snapshots["easyrsa_migrated"]) > 0 {
		migrated = true
	}

	// if pvc does not exist then no migration is required
	if len(input.Snapshots["openvpn_pvc"]) == 0 {
		migrated = true
	} else {
		// if pvc exists, then get storageClassName from it and set effectiveStorageClass
		pvc := input.Snapshots["openvpn_pvc"][0].(string)
		input.Values.Set("openvpn.internal.effectiveStorageClass", pvc)
	}

	statefulsets := input.Snapshots["openvpn_sts"]

	if len(statefulsets) > 0 {
		if migrated && statefulsets[0].(string) != "true" {
			input.PatchCollector.Delete("apps/v1", "StatefulSet", "d8-openvpn", "openvpn")
			input.LogEntry.Infof("statefulset/openvpn deleted (%t/%s)", migrated, statefulsets[0].(string))
		}
	}

	input.Values.Set("openvpn.internal.migrated", migrated)

	return nil
}
