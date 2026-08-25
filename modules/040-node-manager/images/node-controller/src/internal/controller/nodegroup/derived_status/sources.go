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

package derived_status

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/capacity"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	clusterUUIDConfigMapName     = "d8-cluster-uuid"
	clusterUUIDConfigMapNS       = "kube-system"

	clusterKubernetesConfigMapName = "d8-cluster-kubernetes"
	clusterKubernetesConfigMapNS   = "kube-system"

	instanceTypesCatalogName = "for-cluster-autoscaler"
	instanceClassGroup       = "deckhouse.io"

	// InstanceTypesCatalog serves v1alpha1 only, so this one is safe to compile in. The
	// InstanceClass version is not — see common.InstanceClassAPIVersionKey.
	instanceTypesCatalogVersion = "v1alpha1"

	apiserverPodNamespace  = "kube-system"
	apiserverVersionAnnKey = "control-plane-manager.deckhouse.io/kubernetes-version"
)

// isAbsent reports that a source genuinely is not there, as opposed to being unreachable. For
// CRD-backed objects absence has two shapes: the object is missing, or its CRD was never installed
// — a cluster with no MCM has no MachineDeployment kind at all, and a provider that ships no
// instance-type catalog has no InstanceTypesCatalog kind. Both mean "nothing to read", while any
// other failure must reach the caller so the reconcile retries instead of publishing less.
func isAbsent(err error) bool {
	return apierrors.IsNotFound(err) || meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err)
}

// readClusterUUID returns the cluster UUID, which seeds the update-epoch drift. An absent
// ConfigMap is a cluster that has not been stamped yet; an unreadable one is a failure, because a
// silently empty UUID moves every NodeGroup's epoch into the same window.
func (s *Service) readClusterUUID(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterUUIDConfigMapNS, Name: clusterUUIDConfigMapName}, cm)
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read cluster uuid configmap: %w", err)
	}
	return cm.Data["cluster-uuid"], nil
}

type clusterConfiguration struct {
	DefaultCRI string `json:"defaultCRI"`
}

type clusterKubernetesSpec struct {
	DesiredVersion string `json:"desiredVersion"`
}

// readClusterConfiguration returns the target Kubernetes version and the cluster-wide default CRI.
//
// The version no longer comes out of the ClusterConfiguration Secret:
// ClusterConfiguration.kubernetesVersion is deprecated and knows nothing about the ModuleConfig
// setting, so re-deriving it here would disagree with the control plane. It comes from the cluster
// ConfigMap, which carries the single resolved answer. defaultCRI still comes from the Secret.
func (s *Service) readClusterConfiguration(ctx context.Context) (*semver.Version, string, error) {
	target, err := s.readTargetKubernetesVersion(ctx)
	if err != nil {
		return nil, "", err
	}
	defaultCRI, err := s.readDefaultCRI(ctx)
	if err != nil {
		return nil, "", err
	}
	return target, defaultCRI, nil
}

// An absent ConfigMap yields no version: a managed cluster has none at all, and control-plane-manager
// owns it everywhere else. An unreadable one is a failure, because an empty version drops the clamp
// against the running control plane without a trace. A malformed spec keeps yielding no version: that
// is the shape of the data, not the availability of the source.
func (s *Service) readTargetKubernetesVersion(ctx context.Context) (*semver.Version, error) {
	// Served from the kube-system ConfigMap informer, like the Secret below.
	configMap := &corev1.ConfigMap{}
	err := s.Client.Get(ctx, types.NamespacedName{
		Namespace: clusterKubernetesConfigMapNS,
		Name:      clusterKubernetesConfigMapName,
	}, configMap)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cluster kubernetes configmap: %w", err)
	}

	var spec clusterKubernetesSpec
	if err := sigsyaml.Unmarshal([]byte(configMap.Data["spec"]), &spec); err != nil {
		return nil, nil
	}

	version, err := semver.NewVersion(spec.DesiredVersion)
	if err != nil {
		return nil, nil
	}
	return version, nil
}

// An absent Secret yields an empty CRI; an unreadable one is a failure, because an empty value drops
// the CRI default without a trace. A malformed payload keeps yielding empty: that is the shape of the
// data, not the availability of the source.
func (s *Service) readDefaultCRI(ctx context.Context) (string, error) {
	// Served from the kube-system Secret informer (watch-fresh); a live GET here used to
	// cost hundreds of ms on every derived-status pass during a NodeGroup burst.
	secret := &corev1.Secret{}
	err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}, secret)
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read cluster configuration secret: %w", err)
	}
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	raw, ok := []byte(data["cluster-configuration.yaml"]), data["cluster-configuration.yaml"] != ""
	if !ok {
		return "", nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return "", nil
	}
	return cfg.DefaultCRI, nil
}

// readControlPlaneMinVersion returns the lowest version among the running kube-apiservers,
// taken from the annotation control-plane-manager stamps on their static pod manifests
// (candi/control-plane/kube-apiserver.yaml.tpl) and reads back itself
// (040-control-plane-manager/hooks/effective_kubernetes_version.go:147).
//
// It must be the apiserver version, never the kubelet version of the control-plane Nodes:
// the clamp it feeds decides which kubelet package bashible installs, master NodeGroup
// included, so clamping by kubelet would make the value bound itself. control-plane-manager
// in turn refuses to advance the control plane past the node kubelets, so the two would
// wedge each other and no Kubernetes minor upgrade could ever complete. The apiserver
// legitimately leads kubelet by one minor, which is exactly what this clamp allows.
func (s *Service) readControlPlaneMinVersion(ctx context.Context) (*semver.Version, error) {
	pods := &corev1.PodList{}
	if err := s.Client.List(ctx, pods,
		client.InNamespace(apiserverPodNamespace),
		client.MatchingLabels{"component": "kube-apiserver", "tier": "control-plane"},
	); err != nil {
		return nil, fmt.Errorf("list kube-apiserver pods: %w", err)
	}

	var minVer *semver.Version
	for i := range pods.Items {
		raw, ok := pods.Items[i].GetAnnotations()[apiserverVersionAnnKey]
		if !ok {
			continue
		}
		ver, err := semver.NewVersion(raw)
		if err != nil {
			continue
		}
		if minVer == nil || minVer.GreaterThan(ver) {
			minVer = ver
		}
	}
	return minVer, nil
}

// readDefaultZones returns the zones a NodeGroup spreads over when its spec names none. A failed
// List is returned rather than swallowed: fewer zones is a different published element, and the
// element is hashed into every node's configuration checksum.
func (s *Service) readDefaultZones(ctx context.Context, provider cloudprovider.Provider) ([]string, error) {
	seen := make(map[string]struct{})
	zones := make([]string, 0)
	add := func(z string) {
		if z == "" {
			return
		}
		if _, ok := seen[z]; ok {
			return
		}
		seen[z] = struct{}{}
		zones = append(zones, z)
	}

	mdList := &unstructured.UnstructuredList{}
	mdList.SetGroupVersionKind(ngcommon.MCMMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"))
	if err := s.Client.List(ctx, mdList, client.InNamespace(ngcommon.MachineNamespace)); err != nil && !isAbsent(err) {
		return nil, fmt.Errorf("list machine deployments: %w", err)
	}
	for i := range mdList.Items {
		add(mdList.Items[i].GetAnnotations()["zone"])
	}

	for _, z := range provider.Zones {
		add(z)
	}
	// Sorted because the result is published verbatim in the bashible context: the
	// MachineDeployment List comes back in cache map-iteration order, so an unsorted slice
	// would differ on every pass and rewrite the context Secret (and rebuild every bashible
	// step) for no reason. get_crds does the same via set.Slice().
	sort.Strings(zones)
	return zones, nil
}

// readInstanceClassSpec returns the provider's InstanceClass spec. Typed as a map rather than an
// interface: an unstructured spec is always an object or absent, and the two type assertions the
// interface forced on callers only hid that.
func (s *Service) readInstanceClassSpec(ctx context.Context, version, kind, name string) (map[string]any, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: version, Kind: kind})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get %s %q at %s: %w", kind, name, version, err)
	}
	spec, _ := obj.Object["spec"].(map[string]any)
	return spec, nil
}

// readInstanceTypesCatalog returns the built-in instance types. An absent catalog is a legitimate
// cluster state and yields an empty one; an unreadable catalog is returned, because an empty
// catalog makes the capacity calculation fail, and check #3 then declares the NodeGroup invalid —
// turning a transient API failure into a verdict about the NodeGroup.
func (s *Service) readInstanceTypesCatalog(ctx context.Context) (*capacity.InstanceTypesCatalog, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceTypesCatalogVersion, Kind: "InstanceTypesCatalog"})
	err := s.Client.Get(ctx, types.NamespacedName{Name: instanceTypesCatalogName}, obj)
	if isAbsent(err) {
		return capacity.NewInstanceTypesCatalog(nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read instance types catalog: %w", err)
	}

	raw, ok := obj.Object["instanceTypes"]
	if !ok {
		return capacity.NewInstanceTypesCatalog(nil), nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return capacity.NewInstanceTypesCatalog(nil), nil
	}
	var catalogTypes []capacity.InstanceType
	if err := json.Unmarshal(data, &catalogTypes); err != nil {
		return capacity.NewInstanceTypesCatalog(nil), nil
	}
	return capacity.NewInstanceTypesCatalog(catalogTypes), nil
}
