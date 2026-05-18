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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

type fileWriteResult struct {
	Path    string
	Changed bool
	Diff    string
}

// prepareManifestBytes expands env vars in the template and sets checksum annotations.
// Returns raw manifest bytes
func prepareManifestBytes(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, annotations checksumAnnotations) ([]byte, error) {
	key := component.SecretKey()
	if key == "" {
		return nil, fmt.Errorf("no secret key for component %s", component)
	}

	tpl, ok := secretData[key]
	if !ok {
		return nil, fmt.Errorf("template key %q not found in secret", key)
	}

	expanded := os.ExpandEnv(string(tpl))

	manifest, err := setChecksumAnnotations([]byte(expanded), annotations)
	if err != nil {
		return nil, fmt.Errorf("set checksum annotations: %w", err)
	}

	return manifest, nil
}

// writeStaticPodManifestIfChanged expands env vars in the template, sets checksum annotations and writes the manifest to manifestDir/<component>.yaml atomically.
func writeStaticPodManifestIfChanged(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, annotations checksumAnnotations, manifestDir string) (fileWriteResult, error) {
	manifest, err := prepareManifestBytes(component, secretData, annotations)
	if err != nil {
		return fileWriteResult{}, err
	}

	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	return writeFileIfChanged(filename, manifest, 0o600)
}

// updateChecksumAnnotationsIfChanged reads an existing manifest from disk and updates the given checksum annotations
// Empty strings are not changed. Used for UpdatePKI command where only checksums change, not the template.
func updateChecksumAnnotationsIfChanged(component controlplanev1alpha1.OperationComponent, annotations checksumAnnotations, manifestDir string) (fileWriteResult, error) {
	filename := filepath.Join(manifestDir, component.PodComponentName()+".yaml")
	existing, err := os.ReadFile(filename)
	if err != nil {
		return fileWriteResult{}, fmt.Errorf("read existing manifest %s: %w", filename, err)
	}
	updated, err := setChecksumAnnotations(existing, annotations)
	if err != nil {
		return fileWriteResult{}, fmt.Errorf("set checksum annotations: %w", err)
	}
	return writeFileIfChanged(filename, updated, 0o600)
}

// setChecksumAnnotations parses the manifest as a Pod, sets the given checksum annotations, replaces any existing values and serializes back to YAML
func setChecksumAnnotations(manifestBytes []byte, annotations checksumAnnotations) ([]byte, error) {
	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(manifestBytes, pod); err != nil {
		return nil, fmt.Errorf("unmarshal pod manifest: %w", err)
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	for key, value := range desiredChecksumAnnotations(annotations) {
		pod.Annotations[key] = value
	}

	out, err := yaml.Marshal(pod)
	if err != nil {
		return nil, fmt.Errorf("marshal pod manifest: %w", err)
	}
	return out, nil
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

// removeStaleExtraFiles removes extra-files from disk that belong to this component but no longer present in the secret
func removeStaleExtraFiles(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, extraFilesDir string) []fileWriteResult {
	keys := componentDeps(component).ExtraFileKeys
	if len(keys) == 0 {
		return nil
	}
	results := make([]fileWriteResult, 0, len(keys))
	for _, key := range keys {
		if _, exists := secretData[key]; exists {
			continue
		}
		fileName := strings.TrimPrefix(key, "extra-file-")
		path := filepath.Join(extraFilesDir, fileName)
		content, exists, _ := readFileIfExists(path)
		if !exists {
			continue
		}
		if err := os.Remove(path); err != nil {
			continue
		}
		// forming diff for the removed file with full deleted content
		diff := computeUnifiedDiff(string(content), "", path)
		results = append(results, fileWriteResult{Path: path, Changed: true, Diff: diff})
	}
	return results
}

func writeExtraFilesIfChanged(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, extraFilesDir string) ([]fileWriteResult, error) {
	keys := componentDeps(component).ExtraFileKeys
	if len(keys) == 0 {
		return nil, nil
	}
	return writeSecretExtraFilesIfChanged(secretData, extraFilesDir, keys)
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

func manifestMatchesDesired(op *controlplanev1alpha1.ControlPlaneOperation) (bool, error) {
	podComponent := op.Spec.Component.PodComponentName()
	if podComponent == "" {
		return false, nil
	}

	path := filepath.Join(constants.ManifestsPath, podComponent+".yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read manifest %s: %w", path, err)
	}

	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(content, pod); err != nil {
		return false, fmt.Errorf("unmarshal manifest %s: %w", path, err)
	}

	annotations := pod.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	if op.Spec.DesiredConfigChecksum != "" && annotations[constants.ConfigChecksumAnnotationKey] != op.Spec.DesiredConfigChecksum {
		return false, nil
	}
	if op.Spec.DesiredPKIChecksum != "" && annotations[constants.PKIChecksumAnnotationKey] != op.Spec.DesiredPKIChecksum {
		return false, nil
	}
	if op.Spec.DesiredCAChecksum != "" && annotations[constants.CAChecksumAnnotationKey] != op.Spec.DesiredCAChecksum {
		return false, nil
	}
	if stepWasRenewed(op, controlplanev1alpha1.StepRenewPKICerts) &&
		annotations[constants.CertRenewalIDAnnotationKey] != op.Name {
		return false, nil
	}

	return true, nil
}
