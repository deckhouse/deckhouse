package shared

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// common part for filtering secret with Configuration checksums

// ConfigurationChecksum map of NodeGroup configurations: [nodeGroupName]: <checksum>
type ConfigurationChecksum map[string]string

func ConfigurationChecksumHookConfig() go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:                   "configuration_checksums_secret",
		WaitForSynchronization: pointer.BoolPtr(false),
		ApiVersion:             "v1",
		Kind:                   "Secret",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-instance-manager"},
			},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{"configuration-checksums"},
		},
		FilterFunc: filterChecksumSecret,
	}
}

func filterChecksumSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	data := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		data[k] = string(v)
	}

	return ConfigurationChecksum(data), nil
}
