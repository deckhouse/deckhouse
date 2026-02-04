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

package controlplane

import (
	"control-plane-manager/pkg/constants"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type ManifestGenerator interface {
	GenerateManifest(componentName string, tmpDir string) ([]byte, error)
}

type KubeadmManifestGenerator struct{}

// GenerateManifest KubeadmManifestGenerator generate manifests for components using kubeadm init phase.
func (g *KubeadmManifestGenerator) GenerateManifest(componentName string, tmpDir string) ([]byte, error) {
	return generateTmpManifestWithKubeadm(componentName, tmpDir)
}

// generateTmpManifestWithKubeadm generates manifest for component using kubeadm init phase to "tmp directory + etc/kubernetes/manifests".
// This needs for calculating checksum for each components and referenced files from generated manifests.
func generateTmpManifestWithKubeadm(componentName string, tmpDir string) ([]byte, error) {
	kubernetesDir := filepath.Join(tmpDir, constants.RelativeKubernetesDir)
	patchesDir := filepath.Join(tmpDir, constants.RelativePatchesDir)
	args := []string{"init", "phase"}
	if componentName == "etcd" {
		// kubeadm init phase etcd local --config /tmp/control-plane-manager-123/etc/kubernetes/deckhouse/kubeadm/config.yaml
		args = append(args, "etcd", "local", "--config", patchesDir+"/config.yaml")
	} else {
		// kubeadm init phase control-plane apiserver --config /tmp/control-plane-manager-123/etc/kubernetes/deckhouse/kubeadm/config.yaml
		args = append(args, "control-plane", strings.TrimPrefix(componentName, "kube-"), "--config", patchesDir+"/config.yaml")
	}
	args = append(args, "--rootfs", tmpDir)

	klog.Infof("run kubeadm for %v", componentName)

	c := exec.Command(constants.KubeadmPath, args...)
	out, err := c.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		klog.Infof("%s", s)
	}
	manifestPath := filepath.Join(kubernetesDir, componentName+".yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	return manifestBytes, nil
}
