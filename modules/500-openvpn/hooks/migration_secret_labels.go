/*
Copyright 2025 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	serverCertSecretName = "openvpn-pki-server"
	serverCertNameLabel  = "name"
	serverCertIndexLabel = "index.txt"
	serverCertLabelValue = "server"
	namespace            = "d8-openvpn"
)

type serverCert struct {
	NameLabelExists  bool
	IndexLabelExists bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "openvpn_pki_server",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{serverCertSecretName},
			},
			FilterFunc: applyServerCertSecretFilter,
		},
	},
}, addMissingLabels)

func applyServerCertSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to structured object: %v", err)
	}
	_, labelExist := secret.Labels[serverCertNameLabel]
	_, indexExist := secret.Labels[serverCertIndexLabel]
	return serverCert{
		NameLabelExists:  labelExist,
		IndexLabelExists: indexExist,
	}, err
}

func addMissingLabels(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("openvpn_pki_server")
	if len(snaps) == 0 {
		input.Logger.Warn("Secret openvpn-pki-server not found, skipping")
		return nil
	}

	var sc serverCert
	err := snaps[0].UnmarshalTo(&sc)
	if err != nil {
		return fmt.Errorf("failed to unmarshal openvpn_pki_server: %w", err)
	}

	if sc.NameLabelExists && sc.IndexLabelExists {
		return nil
	}

	labels := map[string]interface{}{}
	if !sc.NameLabelExists {
		labels[serverCertNameLabel] = serverCertLabelValue
	}
	if !sc.IndexLabelExists {
		labels[serverCertIndexLabel] = ""
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	}

	input.PatchCollector.PatchWithMerge(
		patch,
		"v1",
		"Secret",
		namespace,
		serverCertSecretName,
	)

	input.Logger.Info("Patched secret %s/%s with label %s=%s", namespace, serverCertSecretName, serverCertNameLabel, serverCertLabelValue)
	return nil
}
