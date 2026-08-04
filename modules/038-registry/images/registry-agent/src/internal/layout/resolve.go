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

package layout

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Resolver replaces the credential references in a layout with the credentials themselves.
//
// The layout is a cluster-scoped object readable by every node, so it names credentials
// instead of carrying them. Resolution happens here, once, on the way in — and the resolved
// layout is what gets cached, because an agent that has only the reference is an agent that
// cannot authenticate the moment the API server goes away, which is exactly the moment the
// cached copy exists for.
type Resolver struct {
	// Namespace holds the Secrets referenced by a layout.
	//
	// No client of its own, deliberately. It used to have one, handed over at construction —
	// and the agent constructs everything before a node has credentials, so on every node
	// that bootstraps into a cluster the copy here stayed nil while the Source's was replaced
	// the moment credentials appeared. Every layout naming a credential was then refused, for
	// the life of the node, and reported as an unreachable API server. One client, held in one
	// place, cannot fall out of step with itself.
	Namespace string
}

// Resolve fills in every referenced credential in place.
//
// Each Secret is read once however many endpoints point into it: every replica of the cache
// shares one credential, and re-reading it per endpoint would turn one layout into a dozen
// API calls on every reconciliation.
func (r *Resolver) Resolve(
	ctx context.Context, reader client.Client, spec *registryv1alpha1.RegistryNodeSpec,
) error {
	auths := spec.ReferencedAuths()
	if len(auths) == 0 {
		return nil
	}

	if r == nil || reader == nil {
		return fmt.Errorf("the layout references credentials and there is nothing to read them with")
	}

	// Each Secret once, however many endpoints point into it.
	contents := map[string]map[string][]byte{}
	for _, name := range registryv1alpha1.SecretNames(auths) {
		secret := &corev1.Secret{}
		key := types.NamespacedName{Namespace: r.Namespace, Name: name}
		if err := reader.Get(ctx, key, secret); err != nil {
			return fmt.Errorf("reading %s: %w", key, err)
		}
		contents[name] = secret.Data
	}

	return registryv1alpha1.ResolveAuths(auths, contents)
}
