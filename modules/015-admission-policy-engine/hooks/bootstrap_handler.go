/*
Copyright 2022 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// We have to have running gatekeeper-controller-manager deployment for handling ConstraintTemplates and create CRDs for them
// so, based on ready deployment replicas we set the `bootstrapped` flag and create templates only when true

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "gatekeeper_deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"gatekeeper-controller-manager"},
			},
			FilterFunc: filterGatekeeperDeployment,
		},
	},
}, handleGatekeeperBootstrap)

func handleGatekeeperBootstrap(input *go_hook.HookInput) error {
	snap := input.Snapshots["gatekeeper_deployment"]
	if len(snap) == 0 {
		input.Values.Set("admissionPolicyEngine.internal.bootstrapped", false)
		return nil
	}

	flag, ok := input.Values.GetOk("admissionPolicyEngine.internal.bootstrapped")
	if ok {
		if flag.Bool() {
			// to prevent flapping
			return nil
		}
	}

	deploymentReady := snap[0].(bool)
	input.Values.Set("admissionPolicyEngine.internal.bootstrapped", deploymentReady)

	return nil
}

func filterGatekeeperDeployment(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dep v1.Deployment

	err := sdk.FromUnstructured(obj, &dep)
	if err != nil {
		return nil, err
	}

	return dep.Status.ReadyReplicas > 0, nil
}
