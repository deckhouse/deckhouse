package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "auth-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn", "d8-user-authz"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane-configurator": "",
				},
			},
			FilterFunc: discoveryFilterSecrets,
		},
	},
}, handleAuthDiscoveryModules)

func discoveryFilterSecrets(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	return discoveryCM{Namespace: cm.Namespace, Data: cm.Data}, nil
}

type discoveryCM struct {
	Namespace string
	Data      map[string]string
}

func handleAuthDiscoveryModules(input *go_hook.HookInput) error {
	snap := input.Snapshots["auth-cm"]
	var authZData, authNData map[string]string

	for _, s := range snap {
		cm := s.(discoveryCM)
		switch cm.Namespace {
		case "d8-user-authn":
			authNData = cm.Data

		case "d8-user-authz":
			authZData = cm.Data
		}
	}

	authzWebhookURLExists := input.ConfigValues.Exists("controlPlaneManager.apiserver.authz.webhookURL")
	authzWebhookCAExists := input.ConfigValues.Exists("controlPlaneManager.apiserver.authz.webhookCA")
	authnOIDCIssuerExists := input.ConfigValues.Exists("controlPlaneManager.apiserver.authn.oidcIssuerURL")
	authnOIDCCAExists := input.ConfigValues.Exists("controlPlaneManager.apiserver.authn.oidcCA")

	if !authzWebhookURLExists && !authzWebhookCAExists {
		// nothing was configured by hand
		if len(authZData) > 0 {
			input.Values.Set("controlPlaneManager.apiserver.authz.webhookURL", authZData["url"])
			input.Values.Set("controlPlaneManager.apiserver.authz.webhookCA", authZData["ca"])
		} else {
			input.Values.Remove("controlPlaneManager.apiserver.authz.webhookURL")
			input.Values.Remove("controlPlaneManager.apiserver.authz.webhookCA")
		}
	}

	if !authnOIDCIssuerExists && !authnOIDCCAExists {
		// nothing was configured by hand
		if len(authNData) > 0 {
			input.Values.Set("controlPlaneManager.apiserver.authn.oidcIssuerURL", authNData["oidcIssuerURL"])
			if ca, ok := authNData["oidcCA"]; ok {
				input.Values.Set("controlPlaneManager.apiserver.authn.oidcCA", ca)
			}
		} else {
			input.Values.Remove("controlPlaneManager.apiserver.authn.oidcIssuerURL")
			input.Values.Remove("controlPlaneManager.apiserver.authn.oidcCA")
		}
	}

	return nil
}
