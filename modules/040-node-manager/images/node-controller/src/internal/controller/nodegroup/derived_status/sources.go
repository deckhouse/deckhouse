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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/capacity"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	cloudProviderSecretName        = ngcommon.CloudProviderSecretName
	cloudProviderSecretNamespace   = nodecommon.CloudProviderSecretNamespace
	clusterConfigSecretName        = "d8-cluster-configuration"
	clusterConfigSecretNamespace   = "kube-system"
	clusterKubernetesConfigMapName = "d8-cluster-kubernetes"
	clusterUUIDConfigMapName       = "d8-cluster-uuid"
	clusterUUIDConfigMapNS         = "kube-system"

	instanceTypesCatalogName = "for-cluster-autoscaler"
	instanceClassGroup       = "deckhouse.io"

	// InstanceTypesCatalog serves v1alpha1 only, so this one is safe to compile in. The
	// InstanceClass version is not — see common.InstanceClassAPIVersionKey.
	instanceTypesCatalogVersion = "v1alpha1"

	apiserverPodNamespace  = "kube-system"
	apiserverVersionAnnKey = "control-plane-manager.deckhouse.io/kubernetes-version"
)

func (s *Service) readCloudProviderData(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName}, secret); err != nil {
		return map[string]interface{}{}
	}
	return decodeSecretData(secret.Data)
}

func decodeSecretData(data map[string][]byte) map[string]interface{} {
	res := make(map[string]interface{}, len(data))
	for k, v := range data {
		var val interface{}
		if err := json.Unmarshal(v, &val); err != nil {
			res[k] = string(v)
			continue
		}
		res[k] = val
	}
	return res
}

func (s *Service) readClusterUUID(ctx context.Context) string {
	cm := &corev1.ConfigMap{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterUUIDConfigMapNS, Name: clusterUUIDConfigMapName}, cm); err != nil {
		return ""
	}
	return cm.Data["cluster-uuid"]
}

type clusterConfiguration struct {
	DefaultCRI string `json:"defaultCRI"`
}

type clusterKubernetesSpec struct {
	DesiredVersion string `json:"desiredVersion"`
}

// readClusterConfiguration returns defaultCRI from the ClusterConfiguration Secret.
//
// NOTE(kubernetesVersion-deprecation): keep — defaultCRI still lives on ClusterConfiguration;
// kubernetesVersion is read only from d8-cluster-kubernetes (see readTargetKubernetesVersion).
func (s *Service) readClusterConfiguration(ctx context.Context) string {
	// Served from the kube-system Secret informer (watch-fresh); a live GET here used to
	// cost hundreds of ms on every derived-status pass during a NodeGroup burst.
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}, secret); err != nil {
		return ""
	}
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	raw, ok := []byte(data["cluster-configuration.yaml"]), data["cluster-configuration.yaml"] != ""
	if !ok {
		return ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return ""
	}
	return cfg.DefaultCRI
}

// readTargetKubernetesVersion reads the resolved target from kube-system/d8-cluster-kubernetes
// data.spec.desiredVersion. That block is owned by control-plane-manager; this controller never
// falls back to ModuleConfig or ClusterConfiguration.
//
// A missing ConfigMap is not an error: it means either a cold start before control-plane-manager
// seeded it, or a managed cluster where control-plane-manager is disabled and there is no such
// object at all. Both degrade to the running kube-apiserver version (readControlPlaneMinVersion),
// which is what this controller did before the version moved into the ConfigMap. Returning an
// error here instead aborted Compute before that fallback, and the bashible context Secret was
// then never written for any NodeGroup — node bootstrap stopped cluster-wide.
//
// A ConfigMap that exists with a missing, empty or unparsable spec is a different story: that is
// not a cold start but a broken object, so it still returns an error and the reconciler requeues.
func (s *Service) readTargetKubernetesVersion(ctx context.Context) (*semver.Version, error) {
	configMap := &corev1.ConfigMap{}
	if err := s.Client.Get(ctx, types.NamespacedName{
		Namespace: clusterConfigSecretNamespace,
		Name:      clusterKubernetesConfigMapName,
	}, configMap); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ConfigMap %s/%s: %w", clusterConfigSecretNamespace, clusterKubernetesConfigMapName, err)
	}

	rawSpec, ok := configMap.Data["spec"]
	if !ok || rawSpec == "" {
		return nil, fmt.Errorf("configMap %s/%s has no 'spec' data key yet", clusterConfigSecretNamespace, clusterKubernetesConfigMapName)
	}

	var spec clusterKubernetesSpec
	if err := sigsyaml.Unmarshal([]byte(rawSpec), &spec); err != nil {
		return nil, fmt.Errorf("unmarshal ConfigMap %s/%s 'spec': %w", clusterConfigSecretNamespace, clusterKubernetesConfigMapName, err)
	}
	if spec.DesiredVersion == "" {
		return nil, fmt.Errorf("configMap %s/%s 'spec.desiredVersion' is empty", clusterConfigSecretNamespace, clusterKubernetesConfigMapName)
	}

	version, err := semver.NewVersion(spec.DesiredVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid ConfigMap %s/%s 'spec.desiredVersion' %q: %w",
			clusterConfigSecretNamespace, clusterKubernetesConfigMapName, spec.DesiredVersion, err)
	}
	return version, nil
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
func (s *Service) readControlPlaneMinVersion(ctx context.Context) *semver.Version {
	pods := &corev1.PodList{}
	if err := s.Client.List(ctx, pods,
		client.InNamespace(apiserverPodNamespace),
		client.MatchingLabels{"component": "kube-apiserver", "tier": "control-plane"},
	); err != nil {
		return nil
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
	return minVer
}

func (s *Service) readDefaultZones(ctx context.Context, cloudProvider map[string]interface{}) []string {
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
	if err := s.Client.List(ctx, mdList, client.InNamespace(ngcommon.MachineNamespace)); err == nil {
		for i := range mdList.Items {
			add(mdList.Items[i].GetAnnotations()["zone"])
		}
	}

	switch v := cloudProvider["zones"].(type) {
	case []string:
		for _, z := range v {
			add(z)
		}
	case []interface{}:
		for _, zi := range v {
			if z, ok := zi.(string); ok {
				add(z)
			}
		}
	case string:
		add(v)
	}
	// Sorted because the result is published verbatim in the bashible context: the
	// MachineDeployment List comes back in cache map-iteration order, so an unsorted slice
	// would differ on every pass and rewrite the context Secret (and rebuild every bashible
	// step) for no reason. get_crds does the same via set.Slice().
	sort.Strings(zones)
	return zones
}

// instanceClassAPIVersion returns the version InstanceClass objects are read at. An empty result
// means the provider has not published it yet; see common.InstanceClassAPIVersionKey for why the
// version is data.
func instanceClassAPIVersion(cloudProvider map[string]interface{}) string {
	version, _ := cloudProvider[nodecommon.InstanceClassAPIVersionKey].(string)
	return version
}

func (s *Service) readInstanceClassSpec(ctx context.Context, version, kind, name string) (interface{}, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: version, Kind: kind})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return nil, fmt.Errorf("get %s %q at %s: %w", kind, name, version, err)
	}
	return obj.Object["spec"], nil
}

func (s *Service) readInstanceTypesCatalog(ctx context.Context) *capacity.InstanceTypesCatalog {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceTypesCatalogVersion, Kind: "InstanceTypesCatalog"})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: instanceTypesCatalogName}, obj); err != nil {
		return capacity.NewInstanceTypesCatalog(nil)
	}

	raw, ok := obj.Object["instanceTypes"]
	if !ok {
		return capacity.NewInstanceTypesCatalog(nil)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return capacity.NewInstanceTypesCatalog(nil)
	}
	var catalogTypes []capacity.InstanceType
	if err := json.Unmarshal(data, &catalogTypes); err != nil {
		return capacity.NewInstanceTypesCatalog(nil)
	}
	return capacity.NewInstanceTypesCatalog(catalogTypes)
}
