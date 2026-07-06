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

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/capacity"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	cloudProviderSecretName      = ngcommon.CloudProviderSecretName
	cloudProviderSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	clusterUUIDConfigMapName     = "d8-cluster-uuid"
	clusterUUIDConfigMapNS       = "kube-system"

	instanceTypesCatalogName = "for-cluster-autoscaler"
	instanceClassGroup       = "deckhouse.io"
	instanceClassVersion     = "v1alpha1"
)

// readCloudProviderData mirrors discover_cloud_provider + decodeDataFromSecret:
// it base64-decodes each Secret data value and JSON-unmarshals it, producing the
// same shape as the internal.cloudProvider value tree
// (.type, .machineClassKind, .capiClusterKind, .<provider>, ...).
func (s *Service) readCloudProviderData(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName}, secret); err != nil {
		return map[string]interface{}{}
	}
	return decodeSecretData(secret.Data)
}

// decodeSecretData decodes each Secret value as JSON, falling back to the raw
// string when it is not valid JSON. corev1.Secret.Data is already base64-decoded
// by the client, matching decodeDataFromSecret's post-base64 behaviour.
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
	KubernetesVersion string `json:"kubernetesVersion"`
	DefaultCRI        string `json:"defaultCRI"`
}

// readClusterConfiguration returns the target Kubernetes version (only when it
// is a concrete semver; "Automatic" and empty yield nil) and the default CRI.
func (s *Service) readClusterConfiguration(ctx context.Context) (*semver.Version, string) {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}, secret); err != nil {
		return nil, ""
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return nil, ""
	}

	var target *semver.Version
	if cfg.KubernetesVersion != "" {
		if ver, err := semver.NewVersion(cfg.KubernetesVersion); err == nil {
			target = ver
		}
	}
	return target, cfg.DefaultCRI
}

// readControlPlaneMinVersion lists control-plane nodes and returns the minimum
// kubelet version, mirroring get_crds's controlPlaneMinVersion (derived from
// global.discovery.kubernetesVersions).
func (s *Service) readControlPlaneMinVersion(ctx context.Context) *semver.Version {
	nodeList := &corev1.NodeList{}
	if err := s.Client.List(ctx, nodeList, client.MatchingLabels{"node-role.kubernetes.io/control-plane": ""}); err != nil {
		return nil
	}

	var min *semver.Version
	for i := range nodeList.Items {
		ver, err := semver.NewVersion(nodeList.Items[i].Status.NodeInfo.KubeletVersion)
		if err != nil {
			continue
		}
		if min == nil || min.GreaterThan(ver) {
			min = ver
		}
	}
	return min
}

// readDefaultZones mirrors get_crds's defaultZones: the union of cloud-provider
// secret zones (machine_deployments zones are covered by the controller's own
// MachineDeployment reconciliation and are not re-derived here).
func (s *Service) readDefaultZones(_ context.Context, cloudProvider map[string]interface{}) []string {
	seen := make(map[string]struct{})
	zones := make([]string, 0)
	add := func(z string) {
		if _, ok := seen[z]; ok {
			return
		}
		seen[z] = struct{}{}
		zones = append(zones, z)
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
	return zones
}

// readInstanceClassSpec performs a dynamic GET of the InstanceClass referenced
// by the NodeGroup (kind carried by the NodeGroup itself), returning its spec.
func (s *Service) readInstanceClassSpec(ctx context.Context, kind, name string) (interface{}, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: kind})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return nil, err
	}
	return obj.Object["spec"], nil
}

// readInstanceTypesCatalog fetches the InstanceTypesCatalog object and builds a
// capacity catalog, mirroring get_crds.applyInstanceTypesCatalog.
func (s *Service) readInstanceTypesCatalog(ctx context.Context) *capacity.InstanceTypesCatalog {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: "InstanceTypesCatalog"})
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
