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

// The dex-authenticator of a DexAuthenticator object, which the chart rendered for every object
// before the shared dex-authenticator instances: its Dex client, and in the namespace of the
// object its Service, its routes, its Deployment with the objects around it, its Secret and a copy
// of the registry Secret.
//
// The release that stops rendering them must not prune them: the pods of an object serve it until
// the controller has given its record to an instance and pointed its Service there. The
// controller keeps the Dex client, the Service and the routes, and deletes the rest after that.
//
// The objects are found by the labels the chart put on every one of them, not by the
// DexAuthenticator objects: an object deleted while the module was not running leaves its
// dex-authenticator behind, and the controller removes that too.

const (
	// dexAuthenticatorObjects selects what the chart rendered in the namespaces of the objects,
	// dexAuthenticatorClients their Dex clients.
	dexAuthenticatorObjects = "app=dex-authenticator,heritage=deckhouse,module=user-authn"
	dexAuthenticatorClients = "app=dex,heritage=deckhouse,module=user-authn,deckhouse.io/dex-authenticator-for"
	// dexAuthenticatorFormerClients selects the Dex clients the chart rendered with these labels
	// and without the name of a DexAuthenticator: the client of a DexAuthenticator had no such
	// label while its ID was <name>-<namespace>-dex-authenticator.
	dexAuthenticatorFormerClients = "app=dex,heritage=deckhouse,module=user-authn,!deckhouse.io/dex-authenticator-for"
)

var dexAuthenticatorGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "dexauthenticators"}

// handedToController is the label of the objects of user-authn-controller. An admission policy
// lets its dexclient-controller change a Service or a route only if the object carries the label
// before the change, so the hook puts it on the ones the controller takes over.
var handedToController = map[string]string{"app.kubernetes.io/managed-by": "user-authn-controller"}

func dexAuthenticatorKind(name string, resource schema.GroupVersionResource, namespace, selector string, labels map[string]string) handover.Kind {
	return handover.Kind{
		Name:     name,
		Resource: resource,
		Labels:   labels,
		Rendered: func(ctx context.Context, c metadata.Interface) ([]handover.Object, error) {
			// Only the objects of the release: the Service and the routes the controller makes
			// carry the same labels, and Protect would only skip them.
			return handover.List(ctx, c, resource, namespace, selector, handover.OwnedByRelease)
		},
	}
}

// dexAuthenticatorFormerClient is the Dex client of a DexAuthenticator under the ID it had before
// dex-authenticator-<name>@<namespace>. The pods of the dex-authenticator of an object that was
// rendered with that ID sign users in with it, so it must outlive the release too: without it
// nobody signs in through the object until its Service points at an instance. The controller
// deletes it then, with the pods.
//
// The client carries no name of its DexAuthenticator, so it is found by its name, from the
// DexAuthenticators, among the metadata of the Dex clients read in one list. The labels keep the
// hook away from a client of another kind that happens to have such an ID: the client of a
// kubeconfig is rendered with another app label.
var dexAuthenticatorFormerClient = handover.Kind{
	Name:     "DexAuthenticator OAuth2Client with the former ID",
	Resource: oauth2ClientGVR,
	Labels:   handedToController,
	Rendered: func(ctx context.Context, c metadata.Interface) ([]handover.Object, error) {
		authenticators, err := handover.List(ctx, c, dexAuthenticatorGVR, "", "", nil)
		if err != nil || len(authenticators) == 0 {
			return nil, err
		}
		wanted := make(map[string]struct{}, len(authenticators))
		for _, a := range authenticators {
			wanted[encoding.ToFnvLikeDex(fmt.Sprintf("%s-%s-dex-authenticator", a.Name, a.Namespace))] = struct{}{}
		}
		return handover.List(ctx, c, oauth2ClientGVR, dexNamespace, dexAuthenticatorFormerClients, func(obj handover.Object) bool {
			_, isWanted := wanted[obj.Name]
			return isWanted && handover.OwnedByRelease(obj)
		})
	},
}

var dexAuthenticatorKinds = []handover.Kind{
	dexAuthenticatorKind("DexAuthenticator OAuth2Client", oauth2ClientGVR, dexNamespace, dexAuthenticatorClients, handedToController),
	dexAuthenticatorFormerClient,
	dexAuthenticatorKind("DexAuthenticator Service",
		schema.GroupVersionResource{Version: "v1", Resource: "services"}, "", dexAuthenticatorObjects, handedToController),
	dexAuthenticatorKind("DexAuthenticator Ingress",
		schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, "", dexAuthenticatorObjects, handedToController),
	dexAuthenticatorKind("DexAuthenticator HTTPRoute",
		schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}, "", dexAuthenticatorObjects, handedToController),
	dexAuthenticatorKind("DexAuthenticator Deployment",
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, "", dexAuthenticatorObjects, nil),
	dexAuthenticatorKind("DexAuthenticator PodDisruptionBudget",
		schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}, "", dexAuthenticatorObjects, nil),
	dexAuthenticatorKind("DexAuthenticator VerticalPodAutoscaler",
		schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}, "", dexAuthenticatorObjects, nil),
	dexAuthenticatorKind("DexAuthenticator NetworkPolicy",
		schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, "", dexAuthenticatorObjects, nil),
	// The Secret with the secrets of an object and the copy of the registry Secret of its namespace.
	dexAuthenticatorKind("DexAuthenticator Secret", secretGVR, "", dexAuthenticatorObjects, nil),
}
