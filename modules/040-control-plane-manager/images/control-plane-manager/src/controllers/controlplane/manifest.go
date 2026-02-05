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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type ManifestGenerator interface {
	GenerateManifest(componentName string, tmpDir string) ([]byte, error)
	GenerateCertificates(componentName string, tmpDir string) error
	GenerateKubeconfigs(tmpDir string) error
}

type KubeadmManifestGenerator struct{}

// GenerateManifest KubeadmManifestGenerator generate manifests for components using kubeadm init phase.
func (g *KubeadmManifestGenerator) GenerateManifest(componentName string, tmpDir string) ([]byte, error) {
	return generateTmpManifestWithKubeadm(componentName, tmpDir)
}

// GenerateCertificates generates certificates for components using kubeadm init phase certs.
func (g *KubeadmManifestGenerator) GenerateCertificates(componentName string, tmpDir string) error {
	return generateTmpCertificatesWithKubeadm(componentName, tmpDir)
}

// GenerateKubeconfigs generates kubeconfig files for control-plane components using kubeadm init phase kubeconfig.
func (g *KubeadmManifestGenerator) GenerateKubeconfigs(tmpDir string) error {
	return generateTmpKubeconfigsWithKubeadm(tmpDir)
}

// generateTmpManifestWithKubeadm generates manifest for component using kubeadm init phase to "tmp directory + etc/kubernetes/manifests".
// This needs for calculating checksum for each components and referenced files from generated manifests.
func generateTmpManifestWithKubeadm(componentName string, tmpDir string) ([]byte, error) {
	manifestDir := filepath.Join(tmpDir, constants.RelativeKubernetesDir, "manifests")
	configPath := filepath.Join("/", constants.RelativeKubeadmDir, "config.yaml")
	args := []string{"init", "phase"}
	if componentName == "etcd" {
		// kubeadm init phase etcd local --config /etc/kubernetes/deckhouse/kubeadm/config.yaml --rootfs /tmp/control-plane-manager-123
		args = append(args, "etcd", "local", "--config", configPath)
	} else {
		// kubeadm init phase control-plane apiserver --config /etc/kubernetes/deckhouse/kubeadm/config.yaml --rootfs /tmp/control-plane-manager-123
		args = append(args, "control-plane", strings.TrimPrefix(componentName, "kube-"), "--config", configPath)
	}
	args = append(args, "--rootfs", tmpDir)

	if err := runKubeadmCommand(args, fmt.Sprintf("generate manifest for %s", componentName)); err != nil {
		return nil, err
	}

	manifestPath := filepath.Join(manifestDir, componentName+".yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	return manifestBytes, nil
}

// generateTmpCertificatesWithKubeadm generates certificates for components using kubeadm init phase certs.
// This is needed to generate peer, server, and healthcheck-client certificates for etcd,
// and other certificates for control-plane components in tmpDir before calculating checksums.
func generateTmpCertificatesWithKubeadm(componentName string, tmpDir string) error {
	configPath := filepath.Join("/", constants.RelativeKubeadmDir, "config.yaml")
	var args []string

	switch componentName {
	case "etcd":
		// kubeadm init phase certs etcd-server --config /etc/kubernetes/deckhouse/kubeadm/config.yaml --rootfs /tmp/control-plane-123
		for _, certName := range []string{"etcd-server", "etcd-peer", "etcd-healthcheck-client"} {
			args = []string{"init", "phase", "certs", certName, "--config", configPath, "--rootfs", tmpDir}
			if err := runKubeadmCommand(args, fmt.Sprintf("generate certificate %s", certName)); err != nil {
				return err
			}
		}
	case "kube-apiserver":
		// Generate apiserver certificates
		for _, certName := range []string{"apiserver", "apiserver-kubelet-client", "apiserver-etcd-client", "front-proxy-client"} {
			args = []string{"init", "phase", "certs", certName, "--config", configPath, "--rootfs", tmpDir}
			if err := runKubeadmCommand(args, fmt.Sprintf("generate certificate %s", certName)); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateTmpKubeconfigsWithKubeadm generates kubeconfig files for control-plane components.
// This is needed to generate controller-manager.conf and scheduler.conf files in tmpDir
// before calculating checksums, as these files are referenced in component manifests.
func generateTmpKubeconfigsWithKubeadm(tmpDir string) error {
	configPath := filepath.Join("/", constants.RelativeKubeadmDir, "config.yaml")

	// Generate kubeconfigs for controller-manager and scheduler, other not needed because they are not referenced in control-plane manifests.
	// For real node-operation-controller should generate super-admin.conf/kubelet.conf/admin.conf too.
	for _, componentName := range []string{"controller-manager", "scheduler"} {
		args := []string{"init", "phase", "kubeconfig", componentName, "--config", configPath, "--rootfs", tmpDir}
		if err := runKubeadmCommand(args, fmt.Sprintf("generate kubeconfig for %s", componentName)); err != nil {
			return err
		}
	}

	return nil
}

func runKubeadmCommand(args []string, description string) error {
	klog.Infof("run kubeadm: %s", description)
	start := time.Now()
	c := exec.Command(constants.KubeadmPath, args...)
	out, err := c.CombinedOutput()

	// Always log
	for _, s := range strings.Split(string(out), "\n") {
		if s != "" {
			klog.Infof("%s", s)
		}
	}

	klog.Infof("kubeadm command took %v", time.Since(start))

	if err != nil {
		return fmt.Errorf("%s: %w", description, err)
	}

	return nil
}
