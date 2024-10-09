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

func handleReports(input *go_hook.HookInput, dc dependency.Container) error {
	sn := input.Snapshots["namespaces"]
	if len(sn) == 0 {
		return nil
	}

	k8sClient := dc.MustGetK8sClient()

	return k8sClient.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "aquasecurity.github.io",
		Version:  "v1alpha1",
		Resource: "sbomreports",
	}).Namespace(metav1.NamespaceAll).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
}
