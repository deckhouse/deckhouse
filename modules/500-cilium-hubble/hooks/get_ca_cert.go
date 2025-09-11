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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/cilium-hubble/get-ca-cert",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ca-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"hubble-ca-secret"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cni-cilium"},
				},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterCASecret,
		},
	},
}, getHubbleCACert)

func filterCASecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		Cert: string(sec.Data["ca.crt"]),
		Key:  string(sec.Data["ca.key"]),
	}, nil
}

func getHubbleCACert(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("ca-secret")

	if len(snaps) == 0 {
		return errors.New("secret with hubble CA not found")
	}

	var adm certificate.Certificate
	err := snaps[0].UnmarshalTo(&adm)
	if err != nil {
		return fmt.Errorf("failed to unmarshal ca secret: %w", err)
	}

	input.Values.Set("ciliumHubble.internal.caCert.cert", adm.Cert)
	input.Values.Set("ciliumHubble.internal.caCert.key", adm.Key)

	return nil
}
