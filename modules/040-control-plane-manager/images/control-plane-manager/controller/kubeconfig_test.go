/*
Copyright 2023 Flant JSC

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

package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestLoadAndParseKubeconfig(t *testing.T) {
	k, err := loadKubeconfig("testdata/kubeconfig.conf")
	if err != nil {
		t.Fatal(err)
	}

	if len(k.AuthInfos[0].AuthInfo.ClientCertificateData) == 0 {
		t.Fatal("client certificate data is empty")
	}

	block, _ := pem.Decode(k.AuthInfos[0].AuthInfo.ClientCertificateData)
	if len(block.Bytes) == 0 {
		t.Fatal("cannot pem decode block")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

}

func TestValidateKubeconfig(t *testing.T) {
	err := validateKubeconfig("testdata/kubeconfig.conf", "testdata/kubeconfig_tmp.conf")
	if err != nil {
		if strings.Contains(err.Error(), "is expiring in less than 30 days") {
			if !errors.Is(err, ErrCertExpiringSoon) {
				t.Fatalf("expected remove to be true when certificate is expiring, got %v", err)
			}
			t.Log("Warning: client certificate is expiring soon, kubeconfig will be recreated.")
		} else {
			t.Fatal(err)
		}
	}
}

func TestCheckEtcdManifest(t *testing.T) {
	os.Setenv("D8_TESTS", "yes")
	config = &Config{
		NodeName: "dev-master-0",
		MyIP:     "192.168.199.39",
	}
	err := checkEtcdManifest()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRootKubeconfigSymlink_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".kube", "config")
	adminConfPath := "/etc/kubernetes/admin.conf"

	err := removeRootKubeconfigSymlink(path, adminConfPath)
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}
}

func TestRemoveRootKubeconfigSymlink_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(kubeDir, "config")
	if err := os.WriteFile(path, []byte("user-created-config"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := removeRootKubeconfigSymlink(path, "/etc/kubernetes/admin.conf")
	if err != nil {
		t.Fatalf("expected no error for regular file, got: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("regular file should not have been removed")
	}
}

func TestRemoveRootKubeconfigSymlink_SymlinkToAdminConf(t *testing.T) {
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	adminConfPath := filepath.Join(tmpDir, "admin.conf")
	if err := os.WriteFile(adminConfPath, []byte("admin-kubeconfig"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Resolve real path (macOS: /var -> /private/var) since EvalSymlinks does this
	realAdminConfPath, err := filepath.EvalSymlinks(adminConfPath)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(kubeDir, "config")
	if err := os.Symlink(adminConfPath, path); err != nil {
		t.Fatal(err)
	}

	err = removeRootKubeconfigSymlink(path, realAdminConfPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("symlink to admin.conf should have been removed")
	}

	if _, err := os.Stat(adminConfPath); os.IsNotExist(err) {
		t.Fatal("admin.conf itself should not have been removed")
	}
}

func TestRemoveRootKubeconfigSymlink_SymlinkToOtherTarget(t *testing.T) {
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	otherConfig := filepath.Join(tmpDir, "other.conf")
	if err := os.WriteFile(otherConfig, []byte("other-config"), 0o644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(kubeDir, "config")
	if err := os.Symlink(otherConfig, path); err != nil {
		t.Fatal(err)
	}

	err := removeRootKubeconfigSymlink(path, filepath.Join(tmpDir, "admin.conf"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, err := os.Lstat(path); os.IsNotExist(err) {
		t.Fatal("symlink to other target should not have been removed")
	}
}

func TestHardenAdminKubeconfigs(t *testing.T) {
	// Save and restore the original kubernetesConfigPath
	originalPath := kubernetesConfigPath
	defer func() { kubernetesConfigPath = originalPath }()

	tmpDir := t.TempDir()
	kubernetesConfigPath = tmpDir

	// Create files with permissive permissions
	superAdminPath := filepath.Join(tmpDir, "super-admin.conf")
	if err := os.WriteFile(superAdminPath, []byte("super-admin-kubeconfig"), 0o644); err != nil {
		t.Fatal(err)
	}

	adminPath := filepath.Join(tmpDir, "admin.conf")
	if err := os.WriteFile(adminPath, []byte("admin-kubeconfig"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call the actual function
	if err := hardenAdminKubeconfigs(); err != nil {
		t.Fatalf("hardenAdminKubeconfigs failed: %v", err)
	}

	// Verify permissions were restricted
	for _, name := range []string{"super-admin.conf", "admin.conf"} {
		path := filepath.Join(tmpDir, name)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("expected %s permissions 0600, got %o", name, fi.Mode().Perm())
		}
	}
}

func TestHardenAdminKubeconfigs_AlreadyRestricted(t *testing.T) {
	originalPath := kubernetesConfigPath
	defer func() { kubernetesConfigPath = originalPath }()

	tmpDir := t.TempDir()
	kubernetesConfigPath = tmpDir

	// Create files already with correct permissions
	adminPath := filepath.Join(tmpDir, "admin.conf")
	if err := os.WriteFile(adminPath, []byte("admin-kubeconfig"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Should not fail even if already restricted
	if err := hardenAdminKubeconfigs(); err != nil {
		t.Fatalf("hardenAdminKubeconfigs failed: %v", err)
	}

	fi, err := os.Stat(adminPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("expected permissions 0600, got %o", fi.Mode().Perm())
	}
}

func TestHardenAdminKubeconfigs_MissingFiles(t *testing.T) {
	originalPath := kubernetesConfigPath
	defer func() { kubernetesConfigPath = originalPath }()

	tmpDir := t.TempDir()
	kubernetesConfigPath = tmpDir

	// Should not fail when files don't exist
	if err := hardenAdminKubeconfigs(); err != nil {
		t.Fatalf("hardenAdminKubeconfigs should not fail for missing files: %v", err)
	}
}

func TestNodeAdminKubeconfigEnvParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		envSet   bool
		expected bool
	}{
		{"default when not set", "", false, true},
		{"explicit false", "false", true, false},
		{"explicit true", "true", true, true},
		{"random value treated as enabled", "something", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.NodeAdminKubeconfig = true

			if tt.envSet {
				t.Setenv("NODE_ADMIN_KUBECONFIG", tt.envValue)
			}

			if v, ok := os.LookupEnv("NODE_ADMIN_KUBECONFIG"); ok && v == "false" {
				c.NodeAdminKubeconfig = false
			}

			if c.NodeAdminKubeconfig != tt.expected {
				t.Fatalf("expected NodeAdminKubeconfig=%v, got %v", tt.expected, c.NodeAdminKubeconfig)
			}
		})
	}
}

// TestNodeAdminKubeconfigReversibility verifies that toggling Config.NodeAdminKubeconfig (driven by NODE_ADMIN_KUBECONFIG)
// removes and restores the /root/.kube/config -> admin.conf symlink.
func TestNodeAdminKubeconfigReversibility(t *testing.T) {
	originalKubeConfigPath := kubernetesConfigPath
	originalConfig := config
	defer func() {
		kubernetesConfigPath = originalKubeConfigPath
		config = originalConfig
	}()

	tmpDir := t.TempDir()
	// Resolve real path (macOS: /var -> /private/var) to match EvalSymlinks behavior
	realTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	kubernetesConfigPath = realTmpDir

	adminConfPath := filepath.Join(realTmpDir, "admin.conf")
	if err := os.WriteFile(adminConfPath, []byte("admin-kubeconfig"), 0o600); err != nil {
		t.Fatal(err)
	}

	kubeDir := filepath.Join(realTmpDir, ".kube")
	kubeconfigPath := filepath.Join(kubeDir, "config")

	t.Setenv("HOME", realTmpDir)

	config = &Config{}

	// Phase 1: NODE_ADMIN_KUBECONFIG unset — symlink should be created
	config.NodeAdminKubeconfig = true
	if err := updateRootKubeconfig(); err != nil {
		t.Fatalf("Phase 1 (true): failed to create symlink: %v", err)
	}

	fi, err := os.Lstat(kubeconfigPath)
	if err != nil {
		t.Fatalf("Phase 1 (true): symlink should exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("Phase 1 (true): /root/.kube/config should be a symlink")
	}

	target, _ := filepath.EvalSymlinks(kubeconfigPath)
	realAdminConf, _ := filepath.EvalSymlinks(adminConfPath)
	if target != realAdminConf {
		t.Fatalf("Phase 1 (true): symlink target %q should point to admin.conf %q", target, realAdminConf)
	}

	// Phase 2: NODE_ADMIN_KUBECONFIG=false — symlink should be removed
	config.NodeAdminKubeconfig = false
	if err := updateRootKubeconfig(); err != nil {
		t.Fatalf("Phase 2 (false): failed to remove symlink: %v", err)
	}

	if _, err := os.Lstat(kubeconfigPath); !os.IsNotExist(err) {
		t.Fatal("Phase 2 (false): symlink should have been removed")
	}

	if _, err := os.Stat(adminConfPath); os.IsNotExist(err) {
		t.Fatal("Phase 2 (false): admin.conf must still exist")
	}

	// Phase 3: NODE_ADMIN_KUBECONFIG unset again — symlink should be recreated
	config.NodeAdminKubeconfig = true
	if err := updateRootKubeconfig(); err != nil {
		t.Fatalf("Phase 3 (true again): failed to recreate symlink: %v", err)
	}

	fi, err = os.Lstat(kubeconfigPath)
	if err != nil {
		t.Fatalf("Phase 3 (true again): symlink should exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("Phase 3 (true again): /root/.kube/config should be a symlink")
	}

	target, _ = filepath.EvalSymlinks(kubeconfigPath)
	if target != realAdminConf {
		t.Fatalf("Phase 3 (true again): symlink target %q should point to admin.conf %q", target, realAdminConf)
	}
}
