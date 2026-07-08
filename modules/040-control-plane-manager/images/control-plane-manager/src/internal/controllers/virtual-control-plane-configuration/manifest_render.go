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

package virtualcontrolplaneconfiguration

import (
	"fmt"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// imagesTable mirrors the "images" key of the global config Secret produced by the virtual-control-plane.yaml Helm template.
type imagesTable struct {
	Versioned        map[string]versionedImages `json:"versioned"`
	Fixed            fixedImages                `json:"fixed"` // independent of the Kubernetes version
	RegistryPackages registryPackagesTable      `json:"registrypackages"`
}

type versionedImages struct {
	Apiserver         string `json:"apiserver"`
	ControllerManager string `json:"controllerManager"`
	Scheduler         string `json:"scheduler"`
}

type fixedImages struct {
	Kine               string `json:"kine"`
	KonnectivityServer string `json:"konnectivityServer"`
	KonnectivityAgent  string `json:"konnectivityAgent"`
	Cilium             string `json:"cilium"`
	CiliumOperator     string `json:"ciliumOperator"`
}

type registryPackagesTable struct {
	Versioned map[string]registryPackagesVersioned `json:"versioned"`
	Fixed     registryPackagesFixed                `json:"fixed"`
}

type registryPackagesVersioned struct {
	Kubelet string `json:"kubelet"`
	Crictl  string `json:"crictl"`
}

type registryPackagesFixed struct {
	Containerd string `json:"containerd"`
	TomlMerge  string `json:"tomlMerge"`
	RppGet     string `json:"rppGet"`
}

func renderManifests(globalData map[string][]byte, vcp *controlplanev1alpha1.VirtualControlPlane, apiAdvertiseAddress string) (map[string][]byte, error) {
	table, err := parseImagesTable(globalData)
	if err != nil {
		return nil, err
	}

	versioned, ok := table.Versioned[vcp.Spec.KubernetesVersion]
	if !ok {
		return nil, fmt.Errorf("no images for kubernetes version %q", vcp.Spec.KubernetesVersion)
	}

	replacer := buildManifestReplacer(vcp, versioned, table.Fixed, apiAdvertiseAddress)

	rendered := make(map[string][]byte)
	for key, value := range globalData {
		switch {
		case strings.HasSuffix(key, ".yaml.tpl"), strings.HasSuffix(key, ".sh.tpl"):
			rendered[key] = []byte(replacer.Replace(string(value)))
		case key == "images", key == "cluster-uuid", key == "minget":
			rendered[key] = value
		}
	}

	return rendered, nil
}

func parseImagesTable(globalData map[string][]byte) (imagesTable, error) {
	raw, ok := globalData["images"]
	if !ok {
		return imagesTable{}, fmt.Errorf("config Secret missing %q key", "images")
	}

	var table imagesTable
	if err := yaml.Unmarshal(raw, &table); err != nil {
		return imagesTable{}, fmt.Errorf("parse images table: %w", err)
	}

	return table, nil
}

func buildTargetConfigSecret(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Secret {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualRenderedConfigSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func buildManifestReplacer(vcp *controlplanev1alpha1.VirtualControlPlane, versioned versionedImages, fixed fixedImages, apiAdvertiseAddress string) *strings.Replacer {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return strings.NewReplacer(
		"${VCP_API_VIP}", apiAdvertiseAddress,
		"${IMAGE_KUBE_APISERVER}", versioned.Apiserver,
		"${IMAGE_KUBE_CONTROLLER_MANAGER}", versioned.ControllerManager,
		"${IMAGE_KUBE_SCHEDULER}", versioned.Scheduler,
		"${IMAGE_KINE}", fixed.Kine,
		"${IMAGE_KONNECTIVITY_SERVER}", fixed.KonnectivityServer,
		"${IMAGE_KONNECTIVITY_AGENT}", fixed.KonnectivityAgent,
		"${IMAGE_CILIUM}", fixed.Cilium,
		"${IMAGE_CILIUM_OPERATOR}", fixed.CiliumOperator,
		"${VCP_NAME}", vcp.Name,
		"${NAMESPACE}", namespace,
		"${CLUSTER_DOMAIN}", constants.DefaultTenantClusterDomain,
		"${SERVICE_SUBNET_CIDR}", constants.DefaultTenantServiceSubnetCIDR,
		"${VCP_API_HOST}", apiExposeHost(vcp),
		"${VCP_KONN_HOST}", konnExposeHost(vcp),
		"${VCP_PKG_HOST}", packagesExposeHost(vcp),
	)
}
