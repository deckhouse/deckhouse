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

	deploymentReady := snap[0].(bool)
	if deploymentReady {
		input.Values.Set("admissionPolicyEngine.internal.bootstrapped", true)
	}

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
