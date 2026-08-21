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

package cloudprovider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// GetCatalog reads every provider registered in the cluster.
func GetCatalog(ctx context.Context, r client.Reader) (Catalog, error) {
	all, err := getProviders(ctx, r)
	if err != nil {
		return Catalog{}, err
	}

	defaultProvider, err := getDefaultProvider(ctx, r)
	if err != nil {
		return Catalog{}, err
	}

	return NewCatalog(all, defaultProvider), nil
}

// NewCatalog builds a Catalog from providers already in hand. The default is the registration
// itself, not its name: resolving a name is GetCatalog's job, and it happens once.
func NewCatalog(all []Provider, defaultProvider Provider) Catalog {
	slices.SortFunc(all, func(a, b Provider) int { return strings.Compare(a.Type, b.Type) })
	return Catalog{
		all:             all,
		defaultProvider: defaultProvider,
	}
}

type Catalog struct {
	all             []Provider
	defaultProvider Provider
}

// All returns every provider, ordered by type.
func (c Catalog) All() []Provider {
	return c.all
}

// Default returns the provider of the cluster itself, the one every non-Static NodeGroup runs on.
func (c Catalog) Default() Provider {
	return c.defaultProvider
}

// ByNodeGroup returns the provider a NodeGroup runs on. The verdict on its spec.providerType
// is ValidateNodeGroup.
func (c Catalog) ByNodeGroup(ng *v1.NodeGroup) Provider {
	if ng.Spec.NodeType == v1.NodeTypeStatic {
		return Provider{}
	}
	return c.defaultProvider
}

// InstanceClassGVKs returns the GVK every provider registered its InstanceClass under.
func (c Catalog) InstanceClassGVKs() []schema.GroupVersionKind {
	ret := make([]schema.GroupVersionKind, 0, len(c.all))
	seen := make(map[schema.GroupVersionKind]bool, len(c.all))

	for i := range c.all {
		p := c.all[i]
		if p.InstanceClassKind == "" || p.InstanceClassAPIVersion == "" {
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   v1.GroupVersion.Group,
			Version: p.InstanceClassAPIVersion,
			Kind:    p.InstanceClassKind,
		}

		// A provider that publishes no type is not deduped on load, so the legacy and the
		// per-provider copy can both be here and would start the same watch twice.
		if seen[gvk] {
			continue
		}

		seen[gvk] = true
		ret = append(ret, gvk)
	}

	slices.SortFunc(ret, func(a, b schema.GroupVersionKind) int {
		if c := strings.Compare(a.Version, b.Version); c != 0 {
			return c
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	return ret
}

// RegisteredInstanceClassGVKs is InstanceClassGVKs over the registrations alone: it answers which
// kinds exist without needing the cluster configuration to be readable.
func RegisteredInstanceClassGVKs(ctx context.Context, r client.Reader) ([]schema.GroupVersionKind, error) {
	providers, err := getProviders(ctx, r)
	if err != nil {
		return nil, err
	}
	return NewCatalog(providers, Provider{}).InstanceClassGVKs(), nil
}

// getProviders is the Secret half of GetCatalog, separate so the lazy InstanceClass watch does not
// depend on the cluster configuration being readable.
func getProviders(ctx context.Context, r client.Reader) ([]Provider, error) {
	secrets := &corev1.SecretList{}

	if err := r.List(ctx, secrets,
		client.InNamespace(RegistrationSecretNamespace),
		client.HasLabels{RegistrationSecretLabel},
	); err != nil {
		return nil, fmt.Errorf("list cloud provider registration secrets: %w", err)
	}

	ret := make([]Provider, 0, len(secrets.Items))
	seen := make(map[string]bool, len(secrets.Items))

	for i := range secrets.Items {
		provider := FromSecretData(secrets.Items[i].Data)
		// The two copies of one registration dedup by type. One that publishes no type is kept: it
		// still carries an InstanceClass kind the watches need.
		if provider.Type != "" {
			if seen[provider.Type] {
				continue
			}
			seen[provider.Type] = true
		}

		ret = append(ret, provider)
	}

	return ret, nil
}

// getDefaultProvider returns the provider every non-Static NodeGroup runs on: the registration a
// provider module publishes under the fixed name, next to its per-provider copy. No such Secret
// means no cloud — the cluster configuration is not consulted, so a provider that has not
// registered yet is indistinguishable from a static cluster.
func getDefaultProvider(ctx context.Context, r client.Reader) (Provider, error) {
	secret := &corev1.Secret{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: RegistrationSecretNamespace,
			Name:      RegistrationSecretBaseName,
		},
		secret,
	)
	if apierrors.IsNotFound(err) {
		return Provider{}, nil
	}
	if err != nil {
		return Provider{}, fmt.Errorf("get secret %q: %w", RegistrationSecretBaseName, err)
	}

	return FromSecretData(secret.Data), nil
}

func byType(all []Provider, pType string) (Provider, bool) {
	if pType == "" {
		return Provider{}, false
	}
	pType = strings.ToLower(pType)

	for i := range all {
		if all[i].Type == pType {
			return all[i], true
		}
	}
	return Provider{}, false
}
