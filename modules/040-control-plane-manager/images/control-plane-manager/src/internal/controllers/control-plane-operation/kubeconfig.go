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

// kubeconfigFilesForComponent returns the kubeconfig files that belong to the given component.
// Returns nil for components without kubeconfigs - Etcd, HotReload, PKI.
func kubeconfigFilesForComponent(c controlplanev1alpha1.OperationComponent) []kubeconfig.File {
	switch c {
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return []kubeconfig.File{kubeconfig.Admin, kubeconfig.SuperAdmin}
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		return []kubeconfig.File{kubeconfig.ControllerManager}
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		return []kubeconfig.File{kubeconfig.Scheduler}
	default:
		return nil
	}
}

// renewKubeconfigsForComponent idempotentally generates kubeconfig files belonging to the given component.
// validates existing files (CA bytes, server address, cert expiry) before writing.
func renewKubeconfigsForComponent(
	component controlplanev1alpha1.OperationComponent,
	secretData map[string][]byte,
	pkiDir, kubeconfigDir, advertiseIP string,
) error {
	files := kubeconfigFilesForComponent(component)
	if len(files) == 0 {
		return nil
	}

	algo := string(secretData[constants.SecretKeyEncryptionAlgorithm])
	if algo != "" {
		return kubeconfig.CreateKubeconfigFiles(files,
			kubeconfig.WithCertificatesDir(pkiDir),
			kubeconfig.WithOutDir(kubeconfigDir),
			kubeconfig.WithLocalAPIEndpoint(advertiseIP),
			kubeconfig.WithEncryptionAlgorithm(pkiconstants.EncryptionAlgorithmType(algo)),
		)
	}

	return kubeconfig.CreateKubeconfigFiles(files,
		kubeconfig.WithCertificatesDir(pkiDir),
		kubeconfig.WithOutDir(kubeconfigDir),
		kubeconfig.WithLocalAPIEndpoint(advertiseIP),
	)
}

// needsRootKubeconfig returns true if the component owns admin.conf (need create /root/.kube/config symlink).
func needsRootKubeconfig(c controlplanev1alpha1.OperationComponent) bool {
	return c == controlplanev1alpha1.OperationComponentKubeAPIServer
}

// updateRootKubeconfig ensures /root/.kube/config is a symlink to admin.conf.
func updateRootKubeconfig(kubeconfigDir, homeDir string) error {
	var symlinkPath string
	if homeDir != "" && homeDir != "/" {
		symlinkPath = filepath.Join(homeDir, ".kube", "config")
	} else {
		symlinkPath = "/root/.kube/config"
	}

	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")

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
