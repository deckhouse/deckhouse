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
	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

// Hubble is compiled into cilium agent, that's why we generate certificate for it.
// We need this certificate inside agent to grant hubble-relay scrape data securely.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/cni-cilium/gen-cert",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ca-cert-secret",
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
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterCASecret,
		},
	},
}, generateHubbleCACert)

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

func generateHubbleCACert(input *go_hook.HookInput) error {
	snap := input.Snapshots["ca-cert-secret"]

	if len(snap) > 0 {
		adm := snap[0].(certificate.Certificate)
		input.Values.Set("cniCilium.internal.hubble.certs.ca.cert", adm.Cert)
		input.Values.Set("cniCilium.internal.hubble.certs.ca.key", adm.Key)

		return nil
	}

	const cn = "d8.hubble-ca.cilium.io"
	ca, err := certificate.GenerateCA(input.LogEntry, cn,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithGroups("d8-cni-cilium"),
	)
	if err != nil {
		return errors.Wrap(err, "generate CA failed")
	}

	input.Values.Set("cniCilium.internal.hubble.certs.ca.cert", ca.Cert)
	input.Values.Set("cniCilium.internal.hubble.certs.ca.key", ca.Key)

	return nil
}
