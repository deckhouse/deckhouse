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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/pmezard/go-difflib/difflib"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type fileWriteResult struct {
	Path    string
	Changed bool
	Diff    string
}

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
	_, err := writeStaticPodManifestIfChanged(component, secretData, configChecksum, pkiChecksum, caChecksum, manifestDir)
	return err
}

func writeStaticPodManifestIfChanged(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, configChecksum, pkiChecksum, caChecksum, manifestDir string) (fileWriteResult, error) {
	manifest, err := prepareManifestBytes(component, secretData, configChecksum, pkiChecksum, caChecksum)
	if err != nil {
		return fileWriteResult{}, err
	}

	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	return writeFileIfChanged(filename, manifest, 0o600)
}

// updateChecksumAnnotations reads an existing manifest from disk and updates the given checksum annotations
// Empty strings are not changed. Used for UpdatePKI command where only checksums change, not the template.
func updateChecksumAnnotations(component controlplanev1alpha1.OperationComponent, pkiChecksum, caChecksum, certRenewalID, manifestDir string) error {
	_, err := updateChecksumAnnotationsIfChanged(component, pkiChecksum, caChecksum, certRenewalID, manifestDir)
	return err
}

func updateChecksumAnnotationsIfChanged(component controlplanev1alpha1.OperationComponent, pkiChecksum, caChecksum, certRenewalID, manifestDir string) (fileWriteResult, error) {
	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	existing, err := os.ReadFile(filename)
	if err != nil {
		return fileWriteResult{}, fmt.Errorf("read existing manifest %s: %w", filename, err)
	}
	updated, err := setChecksumAnnotations(existing, "", pkiChecksum, caChecksum, certRenewalID)
	if err != nil {
		return fileWriteResult{}, fmt.Errorf("set checksum annotations: %w", err)
	}
	return writeFileIfChanged(filename, updated, 0o600)
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
	_, err := writeSecretExtraFilesIfChanged(secretData, extraFilesDir, keys)
	return err
}

func writeSecretExtraFilesIfChanged(secretData map[string][]byte, extraFilesDir string, keys []string) ([]fileWriteResult, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(extraFilesDir, 0o700); err != nil {
		return nil, fmt.Errorf("create extra-files dir: %w", err)
	}
	results := make([]fileWriteResult, 0, len(keys))
	for _, key := range keys {
		content, exists := secretData[key]
		if !exists {
			continue
		}
		dstName := strings.TrimPrefix(key, "extra-file-")
		dst := filepath.Join(extraFilesDir, dstName)
		result, err := writeFileIfChanged(dst, content, 0o600)
		if err != nil {
			return nil, fmt.Errorf("write extra-file %s: %w", dstName, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func writeExtraFiles(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, extraFilesDir string) error {
	_, err := writeExtraFilesIfChanged(component, secretData, extraFilesDir)
	return err
}

func writeExtraFilesIfChanged(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, extraFilesDir string) ([]fileWriteResult, error) {
	podName := component.PodComponentName()
	if podName == "" {
		return nil, nil
	}
	return writeSecretExtraFilesIfChanged(secretData, extraFilesDir, checksum.ExtraFileKeysForPodComponent(podName))
}

// writeHotReloadFiles writes config files that kube-apiserver picks up without restart (see checksum.HotReloadChecksumDependsOn).
func writeHotReloadFiles(secretData map[string][]byte, extraFilesDir string) error {
	_, err := writeHotReloadFilesIfChanged(secretData, extraFilesDir)
	return err
}

func writeHotReloadFilesIfChanged(secretData map[string][]byte, extraFilesDir string) ([]fileWriteResult, error) {
	return writeSecretExtraFilesIfChanged(secretData, extraFilesDir, checksum.HotReloadChecksumDependsOn)
}

func writeFileIfChanged(dst string, desired []byte, perm os.FileMode) (fileWriteResult, error) {
	current, exists, err := readFileIfExists(dst)
	if err != nil {
		return fileWriteResult{}, err
	}

	if exists && bytes.Equal(current, desired) {
		return fileWriteResult{Path: dst, Changed: false}, nil
	}

	diff := computeUnifiedDiff(string(current), string(desired), dst)
	if err := writeFileAtomically(dst, desired, perm); err != nil {
		return fileWriteResult{}, err
	}

	return fileWriteResult{
		Path:    dst,
		Changed: true,
		Diff:    diff,
	}, nil
}

func readFileIfExists(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read %s: %w", path, err)
}

func computeUnifiedDiff(oldContent, newContent, filename string) string {
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(normalizeDiffInput(oldContent)),
		B:        difflib.SplitLines(normalizeDiffInput(newContent)),
		FromFile: filename,
		ToFile:   filename + " (new)",
		Context:  3,
	})
	if err != nil {
		return ""
	}
	return diff
}

func normalizeDiffInput(content string) string {
	if content == "" {
		return ""
	}
	if strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}
