// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// expectedPKIFiles is the full set of files that CreatePKIBundle must produce.
var expectedPKIFiles = []string{
	"ca.crt", "ca.key",
	"apiserver.crt", "apiserver.key",
	"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"front-proxy-client.crt", "front-proxy-client.key",
	"etcd/ca.crt", "etcd/ca.key",
	"etcd/server.crt", "etcd/server.key",
	"etcd/peer.crt", "etcd/peer.key",
	"etcd/healthcheck-client.crt", "etcd/healthcheck-client.key",
	"apiserver-etcd-client.crt", "apiserver-etcd-client.key",
	"sa.key", "sa.pub",
}

// caFiles is the subset that must be byte-identical between idempotent calls.
var caFiles = []string{
	"ca.crt", "ca.key",
	"etcd/ca.crt", "etcd/ca.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"sa.key", "sa.pub",
}

func newPKITemplateConfig(clusterDomain, serviceSubnetCIDR string) *config.ControlPlaneTemplateConfig {
	return &config.ControlPlaneTemplateConfig{
		ClusterConfiguration: map[string]interface{}{
			"clusterDomain":     clusterDomain,
			"serviceSubnetCIDR": serviceSubnetCIDR,
		},
	}
}

func TestGeneratePKIArtifacts_CreatesAllFiles(t *testing.T) {
	artifactsDir := t.TempDir()
	cfg := newPKITemplateConfig("cluster.local", "10.96.0.0/12")

	if err := generatePKIArtifacts("master-0", "10.0.0.1", "10.0.0.1", cfg, artifactsDir); err != nil {
		t.Fatalf("generatePKIArtifacts returned an error: %v", err)
	}

	for _, f := range expectedPKIFiles {
		path := filepath.Join(artifactsDir, "pki", f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %q, but it was not found: %v", f, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file %q is empty", f)
		}
	}
}

// TestGeneratePKIArtifacts_Idempotent ensures that repeated calls do not
// regenerate CA material. Leaf certificates are intentionally allowed to
// rotate (the underlying library may re-issue them with a new serial),
// but the CAs MUST stay stable — otherwise every kubelet/apiserver in the
// cluster gets invalidated on every dhctl re-run.
func TestGeneratePKIArtifacts_Idempotent(t *testing.T) {
	artifactsDir := t.TempDir()
	cfg := newPKITemplateConfig("cluster.local", "10.96.0.0/12")

	if err := generatePKIArtifacts("master-0", "10.0.0.1", "10.0.0.1", cfg, artifactsDir); err != nil {
		t.Fatalf("first generatePKIArtifacts call: %v", err)
	}
	before := readArtifactFiles(t, artifactsDir, caFiles)

	if err := generatePKIArtifacts("master-0", "10.0.0.1", "10.0.0.1", cfg, artifactsDir); err != nil {
		t.Fatalf("second generatePKIArtifacts call: %v", err)
	}
	after := readArtifactFiles(t, artifactsDir, caFiles)

	for _, f := range caFiles {
		if string(before[f]) != string(after[f]) {
			t.Errorf("CA file %q changed after a repeated call (idempotency violated)", f)
		}
	}
}

func TestGeneratePKIArtifacts_ApiserverSANContainsServiceIP(t *testing.T) {
	tests := []struct {
		name             string
		serviceCIDR      string
		expectedFirstSvc string
	}{
		{"standard /12", "10.96.0.0/12", "10.96.0.1"},
		{"custom /16", "192.168.0.0/16", "192.168.0.1"},
		{"small /24", "172.20.0.0/24", "172.20.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifactsDir := t.TempDir()
			cfg := newPKITemplateConfig("cluster.local", tt.serviceCIDR)

			if err := generatePKIArtifacts("master-0", "10.0.0.1", "10.0.0.1", cfg, artifactsDir); err != nil {
				t.Fatalf("generatePKIArtifacts: %v", err)
			}

			cert := loadCertificate(t, filepath.Join(artifactsDir, "pki", "apiserver.crt"))

			expected := net.ParseIP(tt.expectedFirstSvc)
			found := false
			for _, ip := range cert.IPAddresses {
				if ip.Equal(expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("apiserver cert SAN does not contain service IP %s; got: %v",
					tt.expectedFirstSvc, cert.IPAddresses)
			}
		})
	}
}

func TestGeneratePKIArtifacts_ValidationErrors(t *testing.T) {
	validCfg := newPKITemplateConfig("cluster.local", "10.96.0.0/12")

	tests := []struct {
		name        string
		nodeName    string
		nodeIP      string
		endpoint    string
		cfg         *config.ControlPlaneTemplateConfig
		artifactDir string
		wantSubstr  string
	}{
		{
			name:        "empty node name",
			nodeName:    "",
			nodeIP:      "10.0.0.1",
			endpoint:    "10.0.0.1",
			cfg:         validCfg,
			artifactDir: t.TempDir(),
			wantSubstr:  "nodeName is empty",
		},
		{
			name:        "empty endpoint",
			nodeName:    "master-0",
			nodeIP:      "10.0.0.1",
			endpoint:    "",
			cfg:         validCfg,
			artifactDir: t.TempDir(),
			wantSubstr:  "controlPlaneEndpoint is empty",
		},
		{
			name:        "invalid IP",
			nodeName:    "master-0",
			nodeIP:      "not-an-ip",
			endpoint:    "10.0.0.1",
			cfg:         validCfg,
			artifactDir: t.TempDir(),
			wantSubstr:  "invalid node IP",
		},
		{
			name:     "missing clusterDomain",
			nodeName: "master-0",
			nodeIP:   "10.0.0.1",
			endpoint: "10.0.0.1",
			cfg: &config.ControlPlaneTemplateConfig{
				ClusterConfiguration: map[string]interface{}{
					"serviceSubnetCIDR": "10.96.0.0/12",
				},
			},
			artifactDir: t.TempDir(),
			wantSubstr:  "clusterDomain",
		},
		{
			name:     "missing serviceSubnetCIDR",
			nodeName: "master-0",
			nodeIP:   "10.0.0.1",
			endpoint: "10.0.0.1",
			cfg: &config.ControlPlaneTemplateConfig{
				ClusterConfiguration: map[string]interface{}{
					"clusterDomain": "cluster.local",
				},
			},
			artifactDir: t.TempDir(),
			wantSubstr:  "serviceSubnetCIDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generatePKIArtifacts(tt.nodeName, tt.nodeIP, tt.endpoint, tt.cfg, tt.artifactDir)
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantSubstr)
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantSubstr, err)
			}
		})
	}
}

// readArtifactFiles reads the listed files from <dir>/pki and returns a map.
func readArtifactFiles(t *testing.T, dir string, files []string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte, len(files))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, "pki", f))
		if err != nil {
			t.Fatalf("failed to read file %q: %v", f, err)
		}
		result[f] = data
	}
	return result
}

func loadCertificate(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatalf("no PEM block in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate %s: %v", path, err)
	}
	return cert
}
