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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
)

// renewKubeconfigsForComponent idempotentally generates kubeconfig files belonging to the given component.
// validates existing files (CA bytes, server address, cert expiry) before writing.
func renewKubeconfigsForComponent(
	component controlplanev1alpha1.OperationComponent,
	secretData map[string][]byte,
	pkiDir, kubeconfigDir, advertiseIP string,
) (bool, error) {
	files := componentDeps(component).KubeconfigFiles
	if len(files) == 0 {
		return false, nil
	}

	algo := string(secretData[constants.SecretKeyEncryptionAlgorithm])
	if algo != "" {
		report, err := kubeconfig.CreateKubeconfigFiles(files,
			kubeconfig.WithCertificatesDir(pkiDir),
			kubeconfig.WithOutDir(kubeconfigDir),
			kubeconfig.WithLocalAPIEndpoint(advertiseIP),
			kubeconfig.WithEncryptionAlgorithm(pkiconstants.EncryptionAlgorithmType(algo)),
		)
		if err != nil {
			return false, err
		}
		return hasRegeneratedKubeconfigs(report), nil
	}

	report, err := kubeconfig.CreateKubeconfigFiles(files,
		kubeconfig.WithCertificatesDir(pkiDir),
		kubeconfig.WithOutDir(kubeconfigDir),
		kubeconfig.WithLocalAPIEndpoint(advertiseIP),
	)
	if err != nil {
		return false, err
	}
	return hasRegeneratedKubeconfigs(report), nil
}

func hasRegeneratedKubeconfigs(report kubeconfig.KubeconfigApplyReport) bool {
	for i := range report.Entries {
		switch report.Entries[i].Action {
		case kubeconfig.KubeconfigActionWrittenCreated, kubeconfig.KubeconfigActionWrittenRegenerated:
			return true
		}
	}
	return false
}

// updateRootKubeconfig ensures the root kubeconfig symlink matches the node policy:
// nodeAdminKubeconfig=true, /root/.kube/config -> admin.conf
// nodeAdminKubeconfig=false, symlink is removed
func updateRootKubeconfig(kubeconfigDir, homeDir string, nodeAdminKubeconfig bool) error {
	var symlinkPath string
	if homeDir != "" && homeDir != "/" {
		symlinkPath = filepath.Join(homeDir, ".kube", "config")
	} else {
		symlinkPath = "/root/.kube/config"
	}

	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")

	if !nodeAdminKubeconfig {
		return removeRootKubeconfigSymlink(symlinkPath, adminConfPath)
	}

	if info, err := os.Lstat(symlinkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(symlinkPath)
			if err == nil && target == adminConfPath {
				return nil
			}
		}
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("remove existing kubeconfig link: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(symlinkPath), 0o750); err != nil {
		return fmt.Errorf("create .kube dir: %w", err)
	}

	return os.Symlink(adminConfPath, symlinkPath)
}

// removeRootKubeconfigSymlink deletes the root kubeconfig symlink only if it is still exists and points to admin.conf
func removeRootKubeconfigSymlink(symlinkPath, adminConfPath string) error {
	fi, err := os.Lstat(symlinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return nil
	}
	if target != adminConfPath {
		return nil
	}
	return os.Remove(symlinkPath)
}

// hardenAdminKubeconfigs restricts file permissions on admin.conf and super-admin.conf to 0600.
// missing files - skipped, files already at 0600 - untouched.
func hardenAdminKubeconfigs(kubeconfigDir string) error {
	for _, name := range []string{"super-admin.conf", "admin.conf"} {
		path := filepath.Join(kubeconfigDir, name)
		fi, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if fi.Mode().Perm() == 0o600 {
			continue
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}
	return nil
}
