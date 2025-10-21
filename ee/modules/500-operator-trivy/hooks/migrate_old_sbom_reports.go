/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// SBOM Reports are not compatible with the new operator-trivy version
// we have to delete all previous versions of reports

// TODO: delete this hook after the 1.68 release

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/operator-trivy/migrate_old_sbom_reports",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-operator-trivy"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "sbom-migrated",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				},
			},
			FilterFunc: applyNamespaceFilter,
		},
	},
}, dependency.WithExternalDependencies(handleReports))

var sbomGVR = schema.GroupVersionResource{
	Group:    "aquasecurity.github.io",
	Version:  "v1alpha1",
	Resource: "sbomreports",
}

func handleReports(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	sn := input.Snapshots.Get("namespaces")
	if len(sn) == 0 {
		return nil
	}

	k8sClient := dc.MustGetK8sClient()

	list, err := k8sClient.Dynamic().Resource(sbomGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// DeleteCollection does not work here, it gives an error:
	// 		"the server could not find the requested resource"
	for _, item := range list.Items {
		err = k8sClient.Dynamic().Resource(sbomGVR).Namespace(item.GetNamespace()).Delete(context.Background(), item.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
