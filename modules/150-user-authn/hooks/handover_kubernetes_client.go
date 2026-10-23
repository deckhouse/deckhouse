/*
Copyright 2026 Flant JSC

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

	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// The kubernetes OAuth2Client, which the chart rendered with trustedPeers from
// userAuthn.internal.dexAuthenticatorCRDs and dexClientCRDs. Pruned, it would be recreated by the
// controller, but until then no peer could obtain a token for kube-apiserver.

var kubernetesClientName = encoding.ToFnvLikeDex("kubernetes")

var kubernetesOAuth2Client = handover.Kind{
	Name:     "kubernetes OAuth2Client",
	Resource: oauth2ClientGVR,
	Rendered: func(context.Context, dynamic.Interface) ([]handover.Ref, error) {
		return []handover.Ref{{Namespace: dexNamespace, Name: kubernetesClientName}}, nil
	},
}
