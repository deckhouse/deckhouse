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
	"strings"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/capacity"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	cloudProviderSecretName       = ngcommon.CloudProviderSecretName
	cloudProviderSecretNamespace  = "kube-system"
	clusterConfigSecretName       = "d8-cluster-configuration"
	clusterConfigSecretNamespace  = "kube-system"
	automaticKubernetesVersion    = "Automatic"
	deckhouseDefaultK8sVersionKey = "deckhouseDefaultKubernetesVersion"
	clusterUUIDConfigMapName      = "d8-cluster-uuid"
	clusterUUIDConfigMapNS        = "kube-system"

	instanceTypesCatalogName = "for-cluster-autoscaler"
	instanceClassGroup       = "deckhouse.io"
	instanceClassVersion     = "v1alpha1"
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
	KubernetesVersion string `json:"kubernetesVersion"`
	DefaultCRI        string `json:"defaultCRI"`
}

func (s *Service) readClusterConfiguration(ctx context.Context) (*semver.Version, string) {
	// Served from the kube-system Secret informer (watch-fresh); a live GET here used to
	// cost hundreds of ms on every derived-status pass during a NodeGroup burst.
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}, secret); err != nil {
		return nil, ""
	}
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	raw, ok := []byte(data["cluster-configuration.yaml"]), data["cluster-configuration.yaml"] != ""
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
	switch {
	case cfg.KubernetesVersion == automaticKubernetesVersion:
		if enc, ok := data[deckhouseDefaultK8sVersionKey]; ok {
			verRaw := []byte(enc)
			if decoded, err := base64.StdEncoding.DecodeString(enc); err == nil {
				verRaw = decoded
			}
			if ver, err := semver.NewVersion(strings.TrimSpace(string(verRaw))); err == nil {
				target = ver
			}
		}
	case cfg.KubernetesVersion != "":
		if ver, err := semver.NewVersion(cfg.KubernetesVersion); err == nil {
			target = ver
		}
	}
	return target, cfg.DefaultCRI
}

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
	return zones
}

// resolveInstanceClassVersion returns the served API version for a cloud InstanceClass
// kind via the RESTMapper's preferred mapping. Providers publish different versions
// (VCD/Dynamix/HuaweiCloud serve only deckhouse.io/v1), so the version must not be
// hardcoded. Falls back to instanceClassVersion when the kind is unknown to the mapper.
func resolveInstanceClassVersion(mapper meta.RESTMapper, kind string) string {
	if mapper == nil {
		return instanceClassVersion
	}
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: instanceClassGroup, Kind: kind})
	if err != nil {
		return instanceClassVersion
	}
	return mapping.GroupVersionKind.Version
}

func (s *Service) readInstanceClassSpec(ctx context.Context, kind, name string) (interface{}, error) {
	obj := &unstructured.Unstructured{}
	version := resolveInstanceClassVersion(s.Client.RESTMapper(), kind)
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: version, Kind: kind})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return nil, err
	}
	return obj.Object["spec"], nil
}

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
