/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// Performs one-time cleanup of SBOM reports from the cluster if .Values.operatorTrivy.disableSBOMGeneration was set to true

const (
	operatorNs   = "d8-operator-trivy"
	cleanUpLabel = "sbom-cleaned-up"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/operator-trivy/cleanup_sbom_reports",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_namespace",
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{operatorNs},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					cleanUpLabel: "true",
				},
			},
			FilterFunc: applyNamespaceFilter,
		},
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"operator-trivy"},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   applyConfigFilter,
		},
	},
}, dependency.WithExternalDependencies(cleanUpReports))

type moduleConfig struct {
	Spec struct {
		Settings struct {
			DisableSBOMGeneration bool `json:"disableSBOMGeneration"`
		} `json:"settings"`
	} `json:"spec"`
}

func applyConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var mc moduleConfig

	err := sdk.FromUnstructured(obj, &mc)
	if err != nil {
		return nil, err
	}

	return mc.Spec.Settings.DisableSBOMGeneration, nil
}

func cleanUpReports(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	var disableSBOM bool

	snaps := input.Snapshots.Get("module_config")
	if len(snaps) > 0 {
		err := snaps[0].UnmarshalTo(&disableSBOM)
		if err != nil {
			return fmt.Errorf("cannot unmarshal module config: %w", err)
		}

		moduleNsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					cleanUpLabel: nil,
				},
			},
		}

		// cleanup isn't required
		if !disableSBOM {
			// remove cleanup label from the namesapce
			input.PatchCollector.PatchWithMerge(moduleNsPatch, "v1", "Namespace", "", operatorNs)
			return nil
		}

		// cleanup was already done
		if len(input.Snapshots.Get("module_namespace")) > 0 {
			return nil
		}

		k8sClient := dc.MustGetK8sClient()

		list, err := k8sClient.Dynamic().Resource(sbomGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, item := range list.Items {
			if err = k8sClient.Dynamic().Resource(sbomGVR).Namespace(item.GetNamespace()).Delete(context.Background(), item.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
		}

		// set cleanup label to true
		moduleNsPatch = map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					cleanUpLabel: "true",
				},
			},
		}
		input.PatchCollector.PatchWithMerge(moduleNsPatch, "v1", "Namespace", "", operatorNs)
	}

	return nil
}
