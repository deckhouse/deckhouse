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
	"path/filepath"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// syncSecretToTmp syncs secret data to tmp directory in control-plane-manager pod in specify folders for manifests, patches, extra files and pki.
func syncSecretToTmp(secret *corev1.Secret, tmpDir string) error {
	pkiDir := filepath.Join(tmpDir, constants.RelativePkiDir)
	kubeadmDir := filepath.Join(tmpDir, constants.RelativeKubeadmDir)
	patchesDir := filepath.Join(tmpDir, constants.RelativePatchesDir)
	extraFilesDir := filepath.Join(tmpDir, constants.RelativeExtraFilesDir)

	if err := os.MkdirAll(pkiDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(patchesDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(extraFilesDir, 0o700); err != nil {
		return err
	}

	for key, content := range secret.Data {
		switch {
		case key == "kubeadm-config.yaml":
			if err := os.WriteFile(
				filepath.Join(kubeadmDir, "config.yaml"),
				content,
				0o600,
			); err != nil {
				return err
			}

		case strings.HasSuffix(key, ".yaml.tpl"):
			name := strings.TrimSuffix(key, ".tpl")
			if err := os.WriteFile(
				filepath.Join(patchesDir, name),
				content,
				0o600,
			); err != nil {
				return err
			}

		case strings.HasPrefix(key, "extra-file-"):
			name := strings.TrimPrefix(key, "extra-file-")
			if err := os.WriteFile(
				filepath.Join(extraFilesDir, name),
				content,
				0o600,
			); err != nil {
				return err
			}

		case secret.Name == constants.PkiSecretName:
			if err := os.WriteFile(
				filepath.Join(pkiDir, key),
				content,
				0o600,
			); err != nil {
				return err
			}
		}
	}

	return nil

}

func buildDesiredControlPlaneConfiguration(cmpSecret *corev1.Secret, pkiSecret *corev1.Secret, generator ManifestGenerator) (*controlplanev1alpha1.ControlPlaneConfiguration, error) {
	// TODO: validate all configs in other function
	pkiChecksum, err := calculatePKIChecksum(pkiSecret)
	if err != nil {
		return &controlplanev1alpha1.ControlPlaneConfiguration{}, err
	}
	tmpDir, err := os.MkdirTemp("", "control-plane-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := syncSecretToTmp(cmpSecret, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to sync secret to tmp: %w", err)
	}

	components := []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"}
	checksums := make(map[string]string)

	for _, component := range components {
		manifest, err := generator.GenerateManifest(component, tmpDir)
		if err != nil {
			return nil, fmt.Errorf("failed to generate manifest for %s: %w", component, err)
		}

		checksum, err := calculateComponentChecksum(manifest, tmpDir)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", component, err)
		}

		checksums[component] = checksum
	}
	return &controlplanev1alpha1.ControlPlaneConfiguration{
		ObjectMeta: ctrl.ObjectMeta{
			Name: constants.ControlPlaneConfigurationName,
		},
		Spec: controlplanev1alpha1.ControlPlaneConfigurationSpec{
			PKIChecksum: pkiChecksum,
			Components: &controlplanev1alpha1.ControlPlaneComponents{
				Etcd: &controlplanev1alpha1.ComponentChecksum{
					Checksum: checksums["etcd"],
				},
				KubeAPIServer: &controlplanev1alpha1.ComponentChecksum{
					Checksum: checksums["kube-apiserver"],
				},
				KubeControllerManager: &controlplanev1alpha1.ComponentChecksum{
					Checksum: checksums["kube-controller-manager"],
				},
				KubeScheduler: &controlplanev1alpha1.ComponentChecksum{
					Checksum: checksums["kube-scheduler"],
				},
			},
		},
	}, nil
}
