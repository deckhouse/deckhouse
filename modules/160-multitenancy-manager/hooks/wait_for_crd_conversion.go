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
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const conversionCRDsSnapshotName = "conversion_crds"

// conversionCRDs are the CRDs this module converts with a shell hook (webhooks/conversion), which
// the webhook-handler of the deckhouse module serves and registers on the CRD.
//
// A CRD that gains a version is installed before any module runs, and the installer keeps the
// conversion the CRD already has in the cluster. On an upgrade from a release where the CRD had one
// version that is the API server default, strategy None, until the new webhook-handler registers its
// hook. With None the API server only rewrites apiVersion: a ClusterResourceGrantPolicy stored as
// v1alpha1 reads as v1alpha2 with its namespace selector in projectSelector, and the controller's
// first status write would store that reading for good. So the Helm release of the module, and the
// new controller with it, waits until every one of these CRDs converts through the webhook.
var conversionCRDs = []string{
	"clusterresourcegrantpolicies.multitenancy.deckhouse.io",
	"projects.deckhouse.io",
	"projecttemplates.deckhouse.io",
}

type crdConversion struct {
	Name     string `json:"name"`
	Strategy string `json:"strategy"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/160-multitenancy-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         conversionCRDsSnapshotName,
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			NameSelector:                 &types.NameSelector{MatchNames: conversionCRDs},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterCRDConversion,
		},
	},
}, waitForCRDConversion)

func filterCRDConversion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := sdk.FromUnstructured(obj, &crd); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	strategy := string(apiextensionsv1.NoneConverter)
	if crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy != "" {
		strategy = string(crd.Spec.Conversion.Strategy)
	}

	return crdConversion{Name: crd.Name, Strategy: strategy}, nil
}

// waitForCRDConversion fails the module run while a conversion CRD is not converted through the
// webhook yet; addon-operator retries the run, and the snapshot follows the CRD. A CRD that is not
// there is not waited for: nothing of it can be read.
func waitForCRDConversion(_ context.Context, input *go_hook.HookInput) error {
	crds, err := sdkobjectpatch.UnmarshalToStruct[crdConversion](input.Snapshots, conversionCRDsSnapshotName)
	if err != nil {
		return fmt.Errorf("unmarshal the %s snapshot: %w", conversionCRDsSnapshotName, err)
	}

	pending := make([]string, 0, len(crds))
	for _, crd := range crds {
		if crd.Strategy != string(apiextensionsv1.WebhookConverter) {
			pending = append(pending, fmt.Sprintf("%s (conversion strategy %s)", crd.Name, crd.Strategy))
		}
	}
	if len(pending) == 0 {
		return nil
	}

	slices.Sort(pending)

	return fmt.Errorf("waiting for the webhook-handler of the deckhouse module to register the conversion webhook of %s: "+
		"until then the API server converts these objects by rewriting apiVersion only", strings.Join(pending, ", "))
}
