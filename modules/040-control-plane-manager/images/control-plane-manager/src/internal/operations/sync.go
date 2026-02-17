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

package operations

import (
	"control-plane-manager/internal/constants"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// syncSecretToTmp syncs secret data to tmp directory in control-plane-manager pod in specify folders for manifests, patches, extra files and pki.
func SyncSecretToTmp(secret *corev1.Secret, tmpDir string) error {
	pkiDir := filepath.Join(tmpDir, constants.RelativePkiDir)
	etcdPkiDir := filepath.Join(pkiDir, "etcd")
	patchesDir := filepath.Join(tmpDir, constants.RelativePatchesDir)
	extraFilesDir := filepath.Join(tmpDir, constants.RelativeExtraFilesDir)

	if err := os.MkdirAll(etcdPkiDir, 0o700); err != nil {
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
		case strings.HasSuffix(key, ".yaml.tpl"):
			// Expand env in manifest templates (e.g., $MY_IP, $NODE_NAME)
			expandedContent := []byte(os.ExpandEnv(string(content)))
			name := strings.TrimSuffix(key, ".tpl")
			if err := os.WriteFile(
				filepath.Join(patchesDir, name),
				expandedContent,
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
			var filePath string
			if strings.HasPrefix(key, "etcd-") {
				name := strings.TrimPrefix(key, "etcd-")
				filePath = filepath.Join(etcdPkiDir, name)
			} else {
				filePath = filepath.Join(pkiDir, key)
			}

			if err := os.WriteFile(
				filePath,
				content,
				0o600,
			); err != nil {
				return err
			}
		}
	}

	return nil

}
