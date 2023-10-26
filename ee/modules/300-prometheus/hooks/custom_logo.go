/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"crypto/md5"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ns     = "d8-monitoring"
	cmName = "whitelabel-custom-logo"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/custom_logo",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "logo-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			FilterFunc: filterLogoCM,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{cmName},
			},
		},
	},
}, customLogoHandler)

func customLogoHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["logo-cm"]
	if len(snap) == 0 || snap[0] == nil {
		input.Values.Set("prometheus.internal.grafana.customLogo.enabled", false)
		input.PatchCollector.Delete("v1", "ConfigMap", ns, cmName, object_patch.InBackground())
		return nil
	}

	logoData := snap[0].(string)

	cm := buildGrafanaLogoCM(logoData)

	md5Sum := md5.Sum([]byte(logoData))

	input.PatchCollector.Create(cm, object_patch.UpdateIfExists())
	input.Values.Set("prometheus.internal.grafana.customLogo.enabled", true)
	input.Values.Set("prometheus.internal.grafana.customLogo.checksum", fmt.Sprintf("%x", md5Sum))

	return nil
}

func filterLogoCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap

	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, err
	}

	logo, ok := cm.Data["grafanaLogo"]
	if !ok {
		return nil, nil
	}

	return logo, nil
}

func buildGrafanaLogoCM(logo string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns,
			Labels: map[string]string{
				"area": "whitelabel",
			},
		},
		Data: map[string]string{
			"grafanaLogo": logo,
		},
	}
}
