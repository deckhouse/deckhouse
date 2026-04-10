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

package controlplaneoperation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// prepareManifestBytes expands env vars in the template and sets checksum annotations.
// Returns raw manifest bytes
func prepareManifestBytes(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, configChecksum, pkiChecksum, caChecksum string) ([]byte, error) {
	key := component.SecretKey()
	if key == "" {
		return nil, fmt.Errorf("no secret key for component %s", component)
	}

	tpl, ok := secretData[key]
	if !ok {
		return nil, fmt.Errorf("template key %q not found in secret", key)
	}

	expanded := os.ExpandEnv(string(tpl))

	manifest, err := setChecksumAnnotations([]byte(expanded), configChecksum, pkiChecksum, caChecksum, "")
	if err != nil {
		return nil, fmt.Errorf("set checksum annotations: %w", err)
	}

	return manifest, nil
}

// writeStaticPodManifest expands env vars in the template, sets checksum annotations and writes the manifest to manifestDir/<component>.yaml atomically.
func writeStaticPodManifest(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, configChecksum, pkiChecksum, caChecksum, manifestDir string) error {
	manifest, err := prepareManifestBytes(component, secretData, configChecksum, pkiChecksum, caChecksum)
	if err != nil {
		return err
	}

	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	return writeFileAtomically(filename, manifest, 0o600)
}

// updateChecksumAnnotations reads an existing manifest from disk and updates the given checksum annotations
// Empty strings are not changed. Used for UpdatePKI command where only checksums change, not the template.
func updateChecksumAnnotations(component controlplanev1alpha1.OperationComponent, pkiChecksum, caChecksum, certRenewalID, manifestDir string) error {
	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	existing, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read existing manifest %s: %w", filename, err)
	}
	updated, err := setChecksumAnnotations(existing, "", pkiChecksum, caChecksum, certRenewalID)
	if err != nil {
		return fmt.Errorf("set checksum annotations: %w", err)
	}
	return writeFileAtomically(filename, updated, 0o600)
}

// setChecksumAnnotations parses the manifest as a Pod, sets the given checksum annotations, replaces any existing values and serializes back to YAML
func setChecksumAnnotations(manifestBytes []byte, configChecksum, pkiChecksum, caChecksum, certRenewalID string) ([]byte, error) {
	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(manifestBytes, pod); err != nil {
		return nil, fmt.Errorf("unmarshal pod manifest: %w", err)
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	if configChecksum != "" {
		pod.Annotations[constants.ConfigChecksumAnnotationKey] = configChecksum
	}
	if pkiChecksum != "" {
		pod.Annotations[constants.PKIChecksumAnnotationKey] = pkiChecksum
	}
	if caChecksum != "" {
		pod.Annotations[constants.CAChecksumAnnotationKey] = caChecksum
	}
	if certRenewalID != "" {
		pod.Annotations[constants.CertRenewalIDAnnotationKey] = certRenewalID
	}

	out, err := yaml.Marshal(pod)
	if err != nil {
		return nil, fmt.Errorf("marshal pod manifest: %w", err)
	}
	return out, nil
}

// writeSecretExtraFiles writes selected secret keys as files under extraFilesDir (names without extra-file- prefix)
func writeSecretExtraFiles(secretData map[string][]byte, extraFilesDir string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := os.MkdirAll(extraFilesDir, 0o700); err != nil {
		return fmt.Errorf("create extra-files dir: %w", err)
	}
	for _, key := range keys {
		content, exists := secretData[key]
		if !exists {
			continue
		}
		dstName := strings.TrimPrefix(key, "extra-file-")
		dst := filepath.Join(extraFilesDir, dstName)
		if err := writeFileAtomically(dst, content, 0o600); err != nil {
			return fmt.Errorf("write extra-file %s: %w", dstName, err)
		}
	}
	return nil
}

// writeExtraFiles writes extra-files that belong to the given component from secret data to the extra-files directory.
func writeExtraFiles(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, extraFilesDir string) error {
	podName := component.PodComponentName()
	if podName == "" {
		return nil
	}
	return writeSecretExtraFiles(secretData, extraFilesDir, checksum.ExtraFileKeysForPodComponent(podName))
}

// writeHotReloadFiles writes config files that kube-apiserver picks up without restart (see checksum.HotReloadChecksumDependsOn).
func writeHotReloadFiles(secretData map[string][]byte, extraFilesDir string) error {
	return writeSecretExtraFiles(secretData, extraFilesDir, checksum.HotReloadChecksumDependsOn)
}

