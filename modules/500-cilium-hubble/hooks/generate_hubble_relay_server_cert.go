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
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cilium-hubble/gen-cert",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "hubble-relay-server-certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"hubble-relay-server-certs"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cni-cilium"},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterAdmissionSecret,
		},
	},
}, generateHubbleRelayServerCert)

func generateHubbleRelayServerCert(input *go_hook.HookInput) error {
	snap := input.Snapshots["hubble-relay-server-certs"]

	if len(snap) > 0 {
		adm := snap[0].(certificate.Certificate)
		input.Values.Set("ciliumHubble.internal.relay.serverCerts.cert", adm.Cert)
		input.Values.Set("ciliumHubble.internal.relay.serverCerts.key", adm.Key)
		input.Values.Set("ciliumHubble.internal.relay.serverCerts.ca", adm.CA)

		return nil
	}

	ca := genCAAuthority(input)

	const cn = "*.hubble-relay.cilium.io"
	tls, err := certificate.GenerateSelfSignedCert(input.LogEntry,
		cn,
		ca,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSANs("*.hubble-relay.cilium.io"),
		certificate.WithSigningDefaultExpiry(87600*time.Hour),
	)
	if err != nil {
		return errors.Wrap(err, "generate Cert failed")
	}

	input.Values.Set("ciliumHubble.internal.relay.serverCerts.cert", tls.Cert)
	input.Values.Set("ciliumHubble.internal.relay.serverCerts.key", tls.Key)
	input.Values.Set("ciliumHubble.internal.relay.serverCerts.ca", tls.CA)

	return nil
}
