/*
Copyright 2021 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

type clusterIssuer struct {
	Email string
}

func applyClusterIssuerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	r := &clusterIssuer{}
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from clusterissuer %s: %v", obj.GetName(), err)
	}
	if !ok {
		return nil, fmt.Errorf("clusterissuer %s has no spec field", obj.GetName())
	}

	email, _, err := unstructured.NestedString(spec, "acme", "email")
	if err != nil {
		return nil, nil
	}

	r.Email = email
	return r, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("clusterissuers"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ClusterIssuers",
			ApiVersion:                   "cert-manager.io/v1",
			Kind:                         "ClusterIssuer",
			ExecuteHookOnSynchronization: ptr.To(false),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
				},
			},
			FilterFunc: applyClusterIssuerFilter,
		},
	},
}, discoverClusterIssuerEmail)

func discoverClusterIssuerEmail(_ context.Context, input *go_hook.HookInput) error {
	configEmail := input.ConfigValues.Get("certManager.email").String()
	if configEmail != "" {
		input.Values.Set("certManager.internal.email", configEmail)
		return nil
	}

	snapshots := input.Snapshots.Get("ClusterIssuers")
	if len(snapshots) == 0 {
		return nil
	}

	var clustIssuer clusterIssuer
	err := snapshots[0].UnmarshalTo(&clustIssuer)
	if err != nil {
		return fmt.Errorf("failed to unmarshal ClusterIssuer snapshot: %w", err)
	}

	if issuerEmail := clustIssuer.Email; len(issuerEmail) > 0 {
		input.Values.Set("certManager.internal.email", issuerEmail)
	}
	return nil
}
