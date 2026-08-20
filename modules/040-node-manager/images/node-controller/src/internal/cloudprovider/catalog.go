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
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// GetCatalog reads every provider registered in the cluster.
func GetCatalog(ctx context.Context, r client.Reader) (Catalog, error) {
	all, err := getProviders(ctx, r)
	if err != nil {
		return Catalog{}, err
	}

	pType, err := getClusterProviderType(ctx, r)
	if err != nil {
		return Catalog{}, err
	}

	// Static
	if pType == "" {
		return NewCatalog(all, Provider{}), nil
	}

	// Cloud
	provider, ok := byType(all, pType)
	if !ok {
		return Catalog{}, fmt.Errorf(
			"registration secret not found for cloud provider %q in cluster configuration",
			pType,
		)
	}

	return NewCatalog(all, provider), nil
}

// NewCatalog builds a Catalog from providers already in hand. The default is the registration
// itself, not its name: resolving a name is Load's job, and it happens once.
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

// ForNodeGroup returns the provider a NodeGroup runs on and the reason its spec.providerType is
// wrong when it is. It performs no I/O.
//
// The provider is returned whether or not the declaration holds: the nodes run where they run, and
// a NodeGroup being torn down still needs the provider whose objects it left behind.
func (c Catalog) ForNodeGroup(ng *v1.NodeGroup) (Provider, error) {
	provider := c.Resolve(ng)
	return provider, declarationError(ng.Spec.ProviderType, provider)
}

// Resolve returns the provider a NodeGroup runs on, without a verdict on its spec.providerType.
//
// It exists for the current migration only and will be deleted.
func (c Catalog) Resolve(ng *v1.NodeGroup) Provider {
	// A Static node lives outside every cloud.
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

// declarationError reports why a NodeGroup's spec.providerType disagrees with the provider it
// resolved to, or nil when the two agree.
//
// The field declares an answer, it does not pick one: leaving it empty is always correct, and
// naming anything other than the resolved provider is a statement about the NodeGroup that a
// retry cannot fix.
func declarationError(ngPType string, provider Provider) error {
	// An empty field declares nothing, and declaring nothing is always correct.
	if ngPType == "" {
		return nil
	}

	switch {
	case isStatic(ngPType):
		if provider.IsStatic() {
			return nil
		}
		return fmt.Errorf(
			"Invalid providerType '%s'. The nodes of this group run in the '%s' cloud. "+
				"Please remove the field or set it to '%s'.",
			ngPType, provider.Type, provider.Type)

	case provider.IsStatic():
		return fmt.Errorf(
			"Invalid providerType '%s'. The nodes of this group run in no cloud. "+
				"Please remove the field or set it to 'None'.",
			ngPType)

	case !strings.EqualFold(ngPType, provider.Type):
		return fmt.Errorf(
			"Invalid providerType '%s'. Expected '%s'. Please update the NodeGroup to name the "+
				"cloud provider its nodes run in.",
			ngPType, provider.Type)
	}

	return nil
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

// getProviders is the Secret half of Load, separate so the lazy InstanceClass watch does not
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

// getClusterProviderType returns ClusterConfiguration.cloud.provider
// from d8-cluster-configuration secret
func getClusterProviderType(ctx context.Context, r client.Reader) (string, error) {
	secret := &corev1.Secret{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: clusterConfigSecretNamespace,
			Name:      clusterConfigSecretName,
		},
		secret,
	)
	if apierrors.IsNotFound(err) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("get secret %q: %w", clusterConfigSecretName, err)
	}

	raw, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return "", fmt.Errorf("secret %q has no %q key", clusterConfigSecretName, clusterConfigSecretKey)
	}

	// Base64-encoded in some installations; plain YAML is never valid base64, so the fallback is safe.
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	var cfg struct {
		ClusterType string `json:"clusterType"`
		Cloud       struct {
			Provider string `json:"provider"`
		} `json:"cloud"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("unmarshal %q: %w", clusterConfigSecretKey, err)
	}

	if cfg.ClusterType == cloudClusterType &&
		cfg.Cloud.Provider == "" {
		return "", fmt.Errorf("%q is %q but names no cloud.provider", clusterConfigSecretKey, cloudClusterType)
	}

	return strings.ToLower(cfg.Cloud.Provider), nil
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
