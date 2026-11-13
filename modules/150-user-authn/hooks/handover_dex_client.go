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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// The credentials Secret and the OAuth2Clients of a DexClient, rendered by the chart from
// userAuthn.internal.dexClientCRDs before user-authn-controller took them over.

const (
	dexClientSecretPrefix = "dex-client-"
	dexNamespace          = "d8-user-authn"
)

var (
	dexClientGVR    = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "dexclients"}
	secretGVR       = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	oauth2ClientGVR = schema.GroupVersionResource{Group: "dex.coreos.com", Version: "v1", Resource: "oauth2clients"}
)

// The chart let secretMetadata override every label of the Secret, so the rendered ones are found
// by name, from the DexClients: among the metadata of the Secrets of the cluster, read in one
// list, not one Secret at a time.
var dexClientSecrets = handover.Kind{
	Name:     "DexClient Secret",
	Resource: secretGVR,
	Rendered: func(ctx context.Context, c metadata.Interface) ([]handover.Object, error) {
		return dexClientObjects(ctx, c, secretGVR, "", func(namespace, name string) []handover.Ref {
			return []handover.Ref{{Namespace: namespace, Name: dexClientSecretPrefix + name}}
		})
	},
}

var dexClientOAuth2Clients = handover.Kind{
	Name:     "DexClient OAuth2Client",
	Resource: oauth2ClientGVR,
	Rendered: func(ctx context.Context, c metadata.Interface) ([]handover.Object, error) {
		return dexClientObjects(ctx, c, oauth2ClientGVR, dexNamespace, func(namespace, name string) []handover.Ref {
			return []handover.Ref{
				{Namespace: dexNamespace, Name: encoding.ToFnvLikeDex(fmt.Sprintf("dex-client-%s@%s", name, namespace))},
				{Namespace: dexNamespace, Name: encoding.ToFnvLikeDex(fmt.Sprintf("dex-client-%s:%s", name, namespace))},
			}
		})
	},
}

// dexClientObjects lists the objects of gvr in namespace (all namespaces when empty) that have a
// name refs gives for a DexClient. Without DexClients there is nothing to list.
func dexClientObjects(ctx context.Context, c metadata.Interface, gvr schema.GroupVersionResource, namespace string, refs func(namespace, name string) []handover.Ref) ([]handover.Object, error) {
	clients, err := handover.List(ctx, c, dexClientGVR, "", "", nil)
	if err != nil || len(clients) == 0 {
		return nil, err
	}
	wanted := make(map[handover.Ref]struct{}, len(clients))
	for _, dc := range clients {
		for _, ref := range refs(dc.Namespace, dc.Name) {
			wanted[ref] = struct{}{}
		}
	}
	return handover.List(ctx, c, gvr, namespace, "", func(obj handover.Object) bool {
		_, isWanted := wanted[obj.Ref]
		return isWanted
	})
}
