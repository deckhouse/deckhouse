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
	"bytes"
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"additional-configs-for-prometheus": "main"},
			},
			FilterFunc: applyConfigSecretFilter,
		},
	},
}, handleConfigRender)

func applyConfigSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	return sec.Data, nil
}

func handleConfigRender(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("secrets")

	var managers, relabels, scrapes = bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil)

	for data, err := range sdkobjectpatch.SnapshotIter[map[string][]byte](snaps) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'secrets' snapshot: %v", err)
		}

		if v, ok := data["alert-managers.yaml"]; ok {
			managers.Write(v)
			managers.WriteString("\n")
		}

		if v, ok := data["alert-relabels.yaml"]; ok {
			relabels.Write(v)
			relabels.WriteString("\n")
		}

		if v, ok := data["scrapes.yaml"]; ok {
			if len(scrapes.Bytes()) > 0 {
				input.MetricsCollector.Set("d8_deprecated_scrape_config", 1, nil)
			}
			scrapes.Write(v)
			scrapes.WriteString("\n")
		}
	}

	sec := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-main-additional-configs",
			Namespace: "d8-monitoring",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "prometheus",
			},
		},
		Data: map[string][]byte{
			"alert-managers.yaml": managers.Bytes(),
			"alert-relabels.yaml": relabels.Bytes(),
			"scrapes.yaml":        scrapes.Bytes(),
		},
		Type: corev1.SecretTypeOpaque,
	}

	input.PatchCollector.CreateOrUpdate(sec)

	return nil
}
