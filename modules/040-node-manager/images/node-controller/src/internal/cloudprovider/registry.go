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

const (
	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigSecretKey  = "cluster-configuration.yaml"
)

// Registry is the set of registrations the cluster publishes, taken once. Every lookup on it is a
// pure function, so a reconcile loads it once and passes it down instead of re-reading per
// NodeGroup — the bashible context writer used to read the provider Secret once per NodeGroup.
type Registry struct {
	registrations []Registration

	// clusterProvider is ClusterConfiguration.cloud.provider, lower-cased. CloudPermanent
	// NodeGroups resolve through it: they name no InstanceClass, so there is nothing else to match
	// on.
	clusterProvider string
}

// NewRegistry builds a Registry from registrations already in hand, for callers that resolved
// them some other way and for tests that have no cluster to read.
func NewRegistry(registrations []Registration, clusterProvider string) Registry {
	return Registry{registrations: registrations, clusterProvider: strings.ToLower(clusterProvider)}
}

// Load reads every registration in the cluster. It is the only I/O in this package.
//
// An empty result means the cluster has no cloud provider, which is a legitimate state; a failed
// read is returned, because an empty registration is indistinguishable from "no cloud" downstream
// and would publish NodeGroups without instanceClass — a checksum shift on every node.
func Load(ctx context.Context, r client.Reader) (Registry, error) {
	registrations, err := loadRegistrations(ctx, r)
	if err != nil {
		return Registry{}, err
	}
	provider, err := readClusterProvider(ctx, r)
	if err != nil {
		return Registry{}, err
	}
	return Registry{registrations: registrations, clusterProvider: provider}, nil
}

// loadRegistrations is the Secret half of Load, separate so the lazy InstanceClass watch can
// refresh the registered kinds without also depending on the cluster configuration being readable.
func loadRegistrations(ctx context.Context, r client.Reader) ([]Registration, error) {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets,
		client.InNamespace(SecretNamespace),
		client.HasLabels{SecretLabel},
	); err != nil {
		return nil, fmt.Errorf("list cloud provider registration secrets: %w", err)
	}

	ret := make([]Registration, 0, len(secrets.Items))
	seen := make(map[string]bool, len(secrets.Items))
	for i := range secrets.Items {
		decoded := Decode(secrets.Items[i].Data)
		// Providers publish the same registration twice, under the legacy fixed name and under
		// the per-provider one. The type is what identifies a provider, so it is the dedup key —
		// but a registration that publishes none is kept rather than dropped: it still carries an
		// InstanceClass kind the watches need, and dropping it would silently stop watching.
		if decoded.Type != "" {
			if seen[decoded.Type] {
				continue
			}
			seen[decoded.Type] = true
		}
		ret = append(ret, decoded)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Type < ret[j].Type })
	return ret, nil
}

// All returns every registration, ordered by provider type.
func (reg Registry) All() []Registration { return reg.registrations }

// Empty reports a cluster with no cloud provider registered.
func (reg Registry) Empty() bool { return len(reg.registrations) == 0 }

// ByName returns the registration of a provider, matching case-insensitively:
// ClusterConfiguration spells providers OpenStack and vSphere, registrations spell them openstack
// and vsphere.
func (reg Registry) ByName(name string) (Registration, bool) {
	name = strings.ToLower(name)
	for i := range reg.registrations {
		if reg.registrations[i].Type == name {
			return reg.registrations[i], true
		}
	}
	return Registration{}, false
}

// ByInstanceClassKind returns the provider that registered a kind of InstanceClass.
func (reg Registry) ByInstanceClassKind(kind string) (Registration, bool) {
	if kind == "" {
		return Registration{}, false
	}
	for i := range reg.registrations {
		if reg.registrations[i].InstanceClassKind == kind {
			return reg.registrations[i], true
		}
	}
	return Registration{}, false
}

// InstanceClassKinds returns every kind of InstanceClass the cluster accepts, ordered by provider
// type. A NodeGroup referencing a kind outside this set names no provider at all.
func (reg Registry) InstanceClassKinds() []string {
	kinds := make([]string, 0, len(reg.registrations))
	for i := range reg.registrations {
		if reg.registrations[i].InstanceClassKind != "" {
			kinds = append(kinds, reg.registrations[i].InstanceClassKind)
		}
	}
	return kinds
}

// ForNodeGroup returns the provider a NodeGroup runs on. It performs no I/O.
//
// CloudEphemeral names its provider through the InstanceClass kind it references. CloudPermanent
// names none — its nodes are created by the installer, not by this cluster — so it falls back to
// the provider the cluster was configured with. Static and CloudStatic have no provider at all:
// their nodes exist outside any cloud, and handing them one is what used to apply cloud bashible
// steps to bare-metal nodes.
func (reg Registry) ForNodeGroup(ng *v1.NodeGroup) (Registration, bool) {
	switch ng.Spec.NodeType {
	case v1.NodeTypeCloudEphemeral, v1.NodeTypeCloudPermanent:
	default:
		return Registration{}, false
	}

	if ng.Spec.CloudInstances != nil {
		if found, ok := reg.ByInstanceClassKind(ng.Spec.CloudInstances.ClassReference.Kind); ok {
			return found, true
		}
	}

	if ng.Spec.NodeType == v1.NodeTypeCloudPermanent {
		return reg.ByName(reg.clusterProvider)
	}
	return Registration{}, false
}

// InstanceClassGVKs returns the GVK every provider registered its InstanceClass under: the
// instanceClassKind it names at the instanceClassAPIVersion it declares.
//
// A registration without the version contributes nothing — guessing a version is what this whole
// mechanism exists to prevent (see InstanceClassAPIVersionKey). The CRD may lag the Secret:
// callers hand the GVK to a watch that waits for it (source.Kind retries an unserved kind
// itself), they must not assume it is served.
func (reg Registry) InstanceClassGVKs() []schema.GroupVersionKind {
	gvks := make([]schema.GroupVersionKind, 0, len(reg.registrations))
	seen := make(map[schema.GroupVersionKind]bool, len(reg.registrations))
	for i := range reg.registrations {
		r := reg.registrations[i]
		if r.InstanceClassKind == "" || r.InstanceClassAPIVersion == "" {
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   v1.GroupVersion.Group,
			Version: r.InstanceClassAPIVersion,
			Kind:    r.InstanceClassKind,
		}
		// A registration that publishes no type is not deduped on load, so the legacy and the
		// per-provider copy can both be here and would start the same watch twice.
		if seen[gvk] {
			continue
		}
		seen[gvk] = true
		gvks = append(gvks, gvk)
	}
	sort.Slice(gvks, func(i, j int) bool {
		if gvks[i].Version != gvks[j].Version {
			return gvks[i].Version < gvks[j].Version
		}
		return gvks[i].Kind < gvks[j].Kind
	})
	return gvks
}

// readClusterProvider returns ClusterConfiguration.cloud.provider. An absent or unparseable
// configuration yields an empty name: a cluster may legitimately have no cloud section, and the
// only thing that resolves through this value is the CloudPermanent fallback, which then finds
// nothing and reports itself as such.
//
// The secret is parsed here rather than through internal/clusterprefix: that package reads it for
// cloud.prefix and fails closed, because an empty prefix there deletes real MachineDeployments.
func readClusterProvider(ctx context.Context, r client.Reader) (string, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Namespace: SecretNamespace, Name: clusterConfigSecretName}, secret)
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read cluster configuration secret: %w", err)
	}

	raw, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return "", nil
	}
	// Stored base64-encoded in some installations; plain YAML is never valid base64, so falling
	// back to the raw bytes never corrupts an already-decoded document.
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	var cfg struct {
		Cloud struct {
			Provider string `json:"provider"`
		} `json:"cloud"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return "", nil
	}
	return strings.ToLower(cfg.Cloud.Provider), nil
}
