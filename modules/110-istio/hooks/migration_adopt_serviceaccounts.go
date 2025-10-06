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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	labelHelmManagedBy         = "app.kubernetes.io/managed-by"
	annotationReleaseName      = "meta.helm.sh/release-name"
	annotationReleaseNamespace = "meta.helm.sh/release-namespace"
)

//TODO: remove after 1.64

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        lib.Queue("migration-adopt-service-accounts"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istio_serviceaccounts",
			ApiVersion:        "v1",
			Kind:              "ServiceAccount",
			NamespaceSelector: lib.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"istiod"},
					},
				},
			},
			FilterFunc: applyServiceAccountFilter,
		},
	},
}, migrateServiceAccounts)

func applyServiceAccountFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	serviceAccount := &v1.ServiceAccount{}
	err := sdk.FromUnstructured(obj, serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("cannot convert ServiceAccount to struct: %v", err)
	}

	_, isLabeledWithHelmManaged := serviceAccount.Labels[labelHelmManagedBy]
	_, isAnnotatedWithReleseName := serviceAccount.Annotations[annotationReleaseName]
	_, isAnnotatedWithReleseNamespace := serviceAccount.Annotations[annotationReleaseNamespace]

	return ServiceAccountInfo{
		IsLabeledAndAnnotated: isLabeledWithHelmManaged && isAnnotatedWithReleseName && isAnnotatedWithReleseNamespace,
		Name:                  serviceAccount.GetName(),
	}, nil
}

func migrateServiceAccounts(_ context.Context, input *go_hook.HookInput) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				labelHelmManagedBy: "Helm",
			},
			"annotations": map[string]string{
				annotationReleaseName:      "istio",
				annotationReleaseNamespace: "d8-system",
			},
		},
	}

	for serviceAccount, err := range sdkobjectpatch.SnapshotIter[ServiceAccountInfo](input.Snapshots.Get("istio_serviceaccounts")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'istio_serviceaccounts' snapshot: %w", err)
		}

		if !serviceAccount.IsLabeledAndAnnotated {
			input.PatchCollector.PatchWithMerge(patch, "v1", "ServiceAccount", "d8-istio", serviceAccount.Name, object_patch.WithIgnoreMissingObject())
		}
	}

	return nil
}

type ServiceAccountInfo struct {
	IsLabeledAndAnnotated bool
	Name                  string
}
