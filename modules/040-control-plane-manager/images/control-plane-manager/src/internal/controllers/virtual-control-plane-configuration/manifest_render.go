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
	Versioned map[string]versionedImages `json:"versioned"`
	Fixed     fixedImages                `json:"fixed"` // independent of the Kubernetes version
}

type versionedImages struct {
	Apiserver         string `json:"apiserver"`
	ControllerManager string `json:"controllerManager"`
	Scheduler         string `json:"scheduler"`
}

type fixedImages struct {
	Kine     string `json:"kine"`
	Postgres string `json:"postgres"`
}

func renderManifests(globalData map[string][]byte, vcp *controlplanev1alpha1.VirtualControlPlane) (map[string][]byte, error) {
	table, err := parseImagesTable(globalData)
	if err != nil {
		return nil, err
	}

	versioned, ok := table.Versioned[vcp.Spec.KubernetesVersion]
	if !ok {
		return nil, fmt.Errorf("no images for kubernetes version %q", vcp.Spec.KubernetesVersion)
	}

	replacer := buildManifestReplacer(vcp, versioned, table.Fixed)

	rendered := make(map[string][]byte)
	for key, value := range globalData {
		if !strings.HasSuffix(key, ".yaml.tpl") {
			continue
		}
		rendered[key] = []byte(replacer.Replace(string(value)))
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
			Name:      namespace + "-config",
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func buildManifestReplacer(vcp *controlplanev1alpha1.VirtualControlPlane, versioned versionedImages, fixed fixedImages) *strings.Replacer {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return strings.NewReplacer(
		"${IMAGE_KUBE_APISERVER}", versioned.Apiserver,
		"${IMAGE_KUBE_CONTROLLER_MANAGER}", versioned.ControllerManager,
		"${IMAGE_KUBE_SCHEDULER}", versioned.Scheduler,
		"${IMAGE_KINE}", fixed.Kine,
		"${IMAGE_POSTGRES}", fixed.Postgres,
		"${VCP_NAME}", vcp.Name,
		"${NAMESPACE}", namespace,
		"${CLUSTER_DOMAIN}", constants.DefaultTenantClusterDomain,
		"${SERVICE_SUBNET_CIDR}", constants.DefaultTenantServiceSubnetCIDR,
	)
}
