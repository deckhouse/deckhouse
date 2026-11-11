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

package dexclient

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// Dex checks allowedConnectors of a client for every grant that names a connector, the token
// exchange included. A TokenExchangeProvider that lists a DexClient in spec.clients is the
// permission of the administrator to exchange its tokens, so a client that limits its providers
// allows the connector of every such provider as well. A client that does not limit them allows
// every connector already and gets no list.

// tokenExchangeConnectors returns the IDs of the connectors of the TokenExchangeProviders that list
// the DexClient name in namespace, sorted. A provider being deleted is left out: its connector is
// being revoked.
func (r *Reconciler) tokenExchangeConnectors(ctx context.Context, name, namespace string) ([]string, error) {
	providers := controller.List(controller.TokenExchangeProviderGVK)
	if err := r.client.List(ctx, providers); err != nil {
		return nil, fmt.Errorf("list TokenExchangeProviders: %w", err)
	}
	var ids []string
	for i := range providers.Items {
		provider := &providers.Items[i]
		if provider.GetDeletionTimestamp() != nil || !slices.Contains(providerClients(provider), types.NamespacedName{Namespace: namespace, Name: name}) {
			continue
		}
		if id, ok := providerConnectorID(provider); ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

// providerConnectorID is the ID of the connector of the provider, which its controller writes to
// status.connectorID: the ID changes after a revocation, so it is read, never rebuilt. A provider
// whose status names no ID has no connector yet and adds nothing; the client is requeued when the
// status is written. An ID that is not one of the provider's own is ignored, as the provider
// controller ignores it: anyone who can write the status could otherwise open the client to the
// connector of another provider.
func providerConnectorID(provider *unstructured.Unstructured) (string, bool) {
	id, _, _ := unstructured.NestedString(provider.Object, "status", "connectorID")
	if id == "" || !naming.IsTokenExchangeConnectorID(provider.GetName(), string(provider.GetUID()), id) {
		return "", false
	}
	return id, true
}

// providerClients are the DexClients spec.clients of the provider lists.
func providerClients(provider *unstructured.Unstructured) []types.NamespacedName {
	items, _, _ := unstructured.NestedSlice(provider.Object, "spec", "clients")
	refs := make([]types.NamespacedName, 0, len(items))
	for _, item := range items {
		ref, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(ref, "dexClient", "name")
		namespace, _, _ := unstructured.NestedString(ref, "dexClient", "namespace")
		if name != "" && namespace != "" {
			refs = append(refs, types.NamespacedName{Namespace: namespace, Name: name})
		}
	}
	return refs
}

// mapTokenExchangeProvider requeues the DexClients a provider lists. An update maps the old and the
// new object both, so a client removed from spec.clients is requeued too.
func mapTokenExchangeProvider(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	refs := providerClients(u)
	requests := make([]reconcile.Request, 0, len(refs))
	for _, ref := range refs {
		requests = append(requests, reconcile.Request{NamespacedName: ref})
	}
	return requests
}

// allowedConnectors is allowedConnectors of the OAuth2Client: the allowedProviders of the client and
// then the connectors of its token exchange providers it does not name already. Without
// allowedProviders the list stays empty, which allows every connector.
func allowedConnectors(c Client) []string {
	if len(c.Spec.AllowedProviders) == 0 {
		return nil
	}
	out := slices.Clone(c.Spec.AllowedProviders)
	for _, id := range c.TokenExchangeConnectors {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
