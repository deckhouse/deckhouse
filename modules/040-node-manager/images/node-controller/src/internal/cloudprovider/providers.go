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
	"sort"
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

// NewProviders builds a Providers from providers already in hand.
func NewProviders(providers []Provider, clusterProvider string) Providers {
	return Providers{
		providers:       providers,
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
// CloudEphemeral resolves through the InstanceClass kind it references. CloudPermanent and
// CloudStatic reference none, so both fall back to the provider of the cluster: their nodes run in
// that cloud, Deckhouse just does not order them. Static has no provider — its nodes are outside
// every cloud.
func (ps Providers) ForNodeGroup(ng *v1.NodeGroup) (Provider, bool) {
	switch ng.Spec.NodeType {
	case v1.NodeTypeCloudEphemeral, v1.NodeTypeCloudPermanent, v1.NodeTypeCloudStatic:
	default:
		return Provider{}, false
	}

	if ng.Spec.CloudInstances != nil {
		if found, ok := ps.byInstanceClassKind(ng.Spec.CloudInstances.ClassReference.Kind); ok {
			return found, true
		}
	}

	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return ps.byName(ps.clusterProvider)
	}

	return Provider{}, false
}

// InstanceClassKinds returns every kind of InstanceClass the cluster accepts, ordered by provider
// type.
func (ps Providers) InstanceClassKinds() []string {
	ret := make([]string, 0, len(ps.providers))

	for i := range ps.providers {
		if ps.providers[i].InstanceClassKind != "" {
			ret = append(ret, ps.providers[i].InstanceClassKind)
		}
	}

	return ret
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

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Version != ret[j].Version {
			return ret[i].Version < ret[j].Version
		}
		return ret[i].Kind < ret[j].Kind
	})
	return ret
}

// byName matches case-insensitively: ClusterConfiguration spells OpenStack, the Secret openstack.
func (ps Providers) byName(name string) (Provider, bool) {
	name = strings.ToLower(name)

	for i := range ps.providers {
		if ps.providers[i].Type == name {
			return ps.providers[i], true
		}
	}

	return Provider{}, false
}

// byInstanceClassKind returns the provider that registered a kind of InstanceClass.
func (ps Providers) byInstanceClassKind(kind string) (Provider, bool) {
	if kind == "" {
		return Provider{}, false
	}

	for i := range ps.providers {
		if ps.providers[i].InstanceClassKind == kind {
			return ps.providers[i], true
		}
	}

	return Provider{}, false
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

	sort.Slice(ret, func(i, j int) bool { return ret[i].Type < ret[j].Type })
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
