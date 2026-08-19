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

// Providers is the set of providers the cluster publishes, taken once. Every lookup on it is pure,
// so a reconcile loads it once and passes it down instead of re-reading per NodeGroup.
type Providers struct {
	providers []Provider

	// ClusterConfiguration.cloud.provider, lower-cased: CloudPermanent and CloudStatic NodeGroups
	// name no InstanceClass, so there is nothing else to match them on.
	clusterProvider string
}

// Load reads every provider registered in the cluster.
//
// An empty result is a legitimate state; a failed read is returned rather than swallowed, because
// downstream it is indistinguishable from "no cloud" and shifts the checksum of every node.
func Load(ctx context.Context, r client.Reader) (Providers, error) {
	providers, err := loadProviders(ctx, r)
	if err != nil {
		return Providers{}, err
	}

	clusterProvider, err := readClusterProvider(ctx, r)
	if err != nil {
		return Providers{}, err
	}

	ret := NewProviders(
		providers,
		clusterProvider,
	)

	if err := ret.Validate(); err != nil {
		return Providers{}, err
	}
	return ret, nil
}

// NewProviders builds a Providers from providers already in hand. It orders them by type, which
// is what makes All deterministic: its result is published verbatim in the bashible context, and
// a reordering there rewrites the Secret on every pass.
func NewProviders(providers []Provider, clusterProvider string) Providers {
	ordered := slices.Clone(providers)
	slices.SortFunc(ordered, func(a, b Provider) int { return strings.Compare(a.Type, b.Type) })

	return Providers{
		providers:       ordered,
		clusterProvider: strings.ToLower(clusterProvider),
	}
}

// Validate reports a cluster whose configured provider published no registration: CloudPermanent
// resolves through that name alone, so the master would render without provider steps.
func (ps Providers) Validate() error {
	if ps.clusterProvider == "" {
		return nil
	}

	if _, ok := ps.byName(ps.clusterProvider); !ok {
		return fmt.Errorf("cloud provider %q of the cluster configuration published no registration secret", ps.clusterProvider)
	}
	return nil
}

// All returns every provider, ordered by type.
func (ps Providers) All() []Provider {
	return ps.providers
}

// Empty reports a cluster with no cloud provider registered.
func (ps Providers) Empty() bool {
	return len(ps.providers) == 0
}

// ForNodeGroup returns the provider a NodeGroup runs on. It performs no I/O.
//
// The cluster runs one cloud, so the answer follows from the node type alone. When several
// providers become possible, the branching appears here and nowhere else: every consumer already
// asks this function rather than reading a Secret of its own.
func (ps Providers) ForNodeGroup(ng *v1.NodeGroup) (Provider, bool) {
	// A Static node lives outside every cloud. The provider steps do not apply to it, and some of
	// them actively fight the configuration it was set up with by hand.
	if ng.Spec.NodeType == v1.NodeTypeStatic {
		return Provider{}, false
	}

	// byName reports no match on an empty name, so a static cluster — which names no provider —
	// resolves to nothing here without a branch of its own.
	return ps.byName(ps.clusterProvider)
}

// InstanceClassGVKs returns the GVK every provider registered its InstanceClass under.
//
// A provider without the version contributes nothing (see InstanceClassAPIVersionKey). The CRD may
// lag the Secret, so callers must not assume the GVK is already served.
func (ps Providers) InstanceClassGVKs() []schema.GroupVersionKind {
	ret := make([]schema.GroupVersionKind, 0, len(ps.providers))
	seen := make(map[schema.GroupVersionKind]bool, len(ps.providers))

	for i := range ps.providers {
		p := ps.providers[i]
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

// byName matches case-insensitively: ClusterConfiguration spells OpenStack, the Secret openstack.
//
// No name matches nothing, even though a registration that published no type carries an empty one:
// "this group named no provider" and "this registration is malformed" must not resolve to the same
// answer.
func (ps Providers) byName(name string) (Provider, bool) {
	if name == "" {
		return Provider{}, false
	}
	name = strings.ToLower(name)

	for i := range ps.providers {
		if ps.providers[i].Type == name {
			return ps.providers[i], true
		}
	}
	return Provider{}, false
}

// DeclarationError reports why a NodeGroup's spec.providerType disagrees with the provider it
// resolved to, or "" when the two agree.
//
// The field declares an answer, it does not pick one: leaving it empty is always correct, and
// naming anything other than the resolved provider is a statement about the NodeGroup that a
// retry cannot fix.
func DeclarationError(declared string, resolved Provider) string {
	switch {
	case declared == "":
		return ""

	case strings.EqualFold(declared, StatusNone):
		if resolved.Type == "" {
			return ""
		}
		return fmt.Sprintf(
			"Invalid providerType '%s'. The nodes of this group run in the '%s' cloud. "+
				"Please remove the field or set it to '%s'.",
			StatusNone, resolved.Type, resolved.Type)

	case resolved.Type == "":
		return fmt.Sprintf(
			"Invalid providerType '%s'. The nodes of this group run in no cloud. "+
				"Please remove the field or set it to '%s'.",
			declared, StatusNone)

	case !strings.EqualFold(declared, resolved.Type):
		return fmt.Sprintf(
			"Invalid providerType '%s'. Expected '%s'. Please update the NodeGroup to name the "+
				"cloud provider its nodes run in.",
			declared, resolved.Type)
	}

	return ""
}

// loadProviders is the Secret half of Load, separate so the lazy InstanceClass watch does not
// depend on the cluster configuration being readable.
func loadProviders(ctx context.Context, r client.Reader) ([]Provider, error) {
	secrets := &corev1.SecretList{}

	if err := r.List(ctx, secrets,
		client.InNamespace(SecretNamespace),
		client.HasLabels{SecretLabel},
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

// readClusterProvider returns ClusterConfiguration.cloud.provider
// from d8-cluster-configuration secret
func readClusterProvider(ctx context.Context, r client.Reader) (string, error) {
	secret := &corev1.Secret{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: SecretNamespace,
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
