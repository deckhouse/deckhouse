/*
Copyright 2025 Flant JSC

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

package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestKubeadmConfigRendering(t *testing.T) {
	InitGlobalVars("/deckhouse")

	t.Run("Version Selection", testVersionSelection)
	t.Run("Feature Gates", testFeatureGates)
	t.Run("API Server Configuration", testAPIServerConfiguration)
	t.Run("Cluster Types", testClusterTypes)
	t.Run("Run Types", testRunTypes)
	t.Run("Service Account Configuration", testServiceAccountConfiguration)
	t.Run("ETCD Configuration", testETCDConfiguration)
	t.Run("Optional Arguments", testOptionalArguments)
	t.Run("Patches Rendering", testPatchesRendering)
	t.Run("Edge Cases", testEdgeCases)
	t.Run("Missing Coverage", testMissingCoverage)
}

func testVersionSelection(t *testing.T) {
	tests := []struct {
		name           string
		k8sVersion     string
		expectedAPI    string
		expectedConfig string
	}{
		{
			name:           "Kubernetes 1.30 should use v1beta3",
			k8sVersion:     "1.30",
			expectedAPI:    "kubeadm.k8s.io/v1beta3",
			expectedConfig: "ClusterConfiguration",
		},
		{
			name:           "Kubernetes 1.31 should use v1beta4",
			k8sVersion:     "1.31",
			expectedAPI:    "kubeadm.k8s.io/v1beta4",
			expectedConfig: "ClusterConfiguration",
		},
		{
			name:           "Kubernetes 1.32 should use v1beta4",
			k8sVersion:     "1.32",
			expectedAPI:    "kubeadm.k8s.io/v1beta4",
			expectedConfig: "ClusterConfiguration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := getBaseTemplateData(tt.k8sVersion)
			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, tt.expectedAPI) {
				t.Errorf("Expected API version %s not found in result", tt.expectedAPI)
			}

			if !strings.Contains(result, tt.expectedConfig) {
				t.Errorf("Expected config kind %s not found in result", tt.expectedConfig)
			}
		})
	}
}

func testFeatureGates(t *testing.T) {
	tests := []struct {
		name             string
		k8sVersion       string
		expectedFeatures []string
	}{
		{
			name:       "Kubernetes 1.30 should not include legacy feature gates",
			k8sVersion: "1.30",
			expectedFeatures: []string{
				"TopologyAwareHints=true",
				"RotateKubeletServerCertificate=true",
			},
		},
		{
			name:       "Kubernetes 1.31 should not include legacy feature gates",
			k8sVersion: "1.31",
			expectedFeatures: []string{
				"TopologyAwareHints=true",
				"RotateKubeletServerCertificate=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := getBaseTemplateData(tt.k8sVersion)
			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			var featureGatesRegex *regexp.Regexp
			if strings.Contains(result, "v1beta3") {
				// v1beta3 uses different format
				featureGatesRegex = regexp.MustCompile(`feature-gates:\s*"([^"]+)"`)
			} else {
				// v1beta4 uses name/value format
				featureGatesRegex = regexp.MustCompile(`- name: feature-gates\s+value:\s*"([^"]+)"`)
			}

			matches := featureGatesRegex.FindStringSubmatch(result)
			if len(matches) < 2 {
				t.Fatalf("Could not find feature-gates in result")
			}

			featureGates := matches[1]
			for _, expected := range tt.expectedFeatures {
				if !strings.Contains(featureGates, expected) {
					t.Errorf("Expected feature gate %s not found in: %s", expected, featureGates)
				}
			}

			if tt.k8sVersion >= "1.30" {
				unexpectedFeatures := []string{
					"ValidatingAdmissionPolicy=true",
					"AdmissionWebhookMatchConditions=true",
					"StructuredAuthenticationConfiguration=true",
				}
				for _, unexpected := range unexpectedFeatures {
					if strings.Contains(featureGates, unexpected) {
						t.Errorf("Unexpected feature gate %s found in: %s", unexpected, featureGates)
					}
				}
			}
		})
	}
}

func testAPIServerConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		k8sVersion string
	}{
		{
			name:       "v1beta3 configuration",
			k8sVersion: "1.30",
		},
		{
			name:       "v1beta4 configuration",
			k8sVersion: "1.31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("Webhook Configuration", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"webhookURL": "https://webhook.example.com",
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "authorization-mode") {
					t.Error("Expected authorization-mode not found")
				}
				if !strings.Contains(result, "Node,Webhook,RBAC") {
					t.Error("Expected webhook authorization mode not found")
				}
				if !strings.Contains(result, "authorization-webhook-config-file") {
					t.Error("Expected webhook config file not found")
				}
			})

			t.Run("Authentication Webhook", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"authnWebhookURL":      "https://authn.example.com",
					"authnWebhookCacheTTL": "10m",
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "authentication-token-webhook-config-file") {
					t.Error("Expected authentication webhook config file not found")
				}
				if !strings.Contains(result, "authentication-token-webhook-cache-ttl") {
					t.Error("Expected authentication webhook cache TTL not found")
				}
				if !strings.Contains(result, "10m") {
					t.Error("Expected cache TTL value not found")
				}
			})

			t.Run("Audit Configuration - File Output", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"auditPolicy": "some-policy",
					"auditLog": map[string]interface{}{
						"output": "File",
						"path":   "/var/log/audit",
					},
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "audit-policy-file") {
					t.Error("Expected audit policy file not found")
				}
				if !strings.Contains(result, "audit-log-path") {
					t.Error("Expected audit log path not found")
				}
				if !strings.Contains(result, "/var/log/kube-audit/audit.log") {
					t.Error("Expected audit log file path not found")
				}
				if !strings.Contains(result, "audit-log-truncate-enabled") {
					t.Error("Expected audit log truncate not found")
				}
			})

			t.Run("Audit Configuration - Stdout Output", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"auditPolicy": "some-policy",
					"auditLog": map[string]interface{}{
						"output": "Stdout",
					},
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				var foundStdout bool
				if strings.Contains(result, "v1beta3") {
					foundStdout = strings.Contains(result, `audit-log-path: "-"`)
				} else {
					foundStdout = strings.Contains(result, `value: "-"`)
				}
				if !foundStdout {
					t.Error("Expected stdout audit log path not found")
				}
			})

			t.Run("Audit Webhook", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"auditWebhookURL": "https://audit.example.com",
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "audit-webhook-config-file") {
					t.Error("Expected audit webhook config file not found")
				}
			})

			t.Run("OIDC Configuration", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"oidcIssuerURL": "https://oidc.example.com",
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "authentication-config") {
					t.Error("Expected authentication config not found")
				}
			})

			t.Run("Bind Address Configuration", func(t *testing.T) {
				bindTests := []struct {
					name         string
					apiserver    map[string]interface{}
					nodeIP       string
					expectedAddr string
				}{
					{
						name: "Bind to wildcard",
						apiserver: map[string]interface{}{
							"bindToWildcard": true,
						},
						expectedAddr: "0.0.0.0",
					},
					{
						name:         "Bind to nodeIP",
						apiserver:    map[string]interface{}{},
						nodeIP:       "192.168.1.100",
						expectedAddr: "192.168.1.100",
					},
					{
						name:         "Default bind address",
						apiserver:    map[string]interface{}{},
						expectedAddr: "127.0.0.1",
					},
				}

				for _, bindTest := range bindTests {
					t.Run(bindTest.name, func(t *testing.T) {
						data := getBaseTemplateData(tt.k8sVersion)
						data["apiserver"] = bindTest.apiserver
						if bindTest.nodeIP != "" {
							data["nodeIP"] = bindTest.nodeIP
						}

						result, err := renderKubeadmConfig(data)
						if err != nil {
							t.Fatalf("Failed to render kubeadm config: %v", err)
						}

						var bindAddrRegex *regexp.Regexp
						if strings.Contains(result, "v1beta3") {
							bindAddrRegex = regexp.MustCompile(`bind-address:\s*"([^"]+)"`)
						} else {
							bindAddrRegex = regexp.MustCompile(`- name: bind-address\s+value:\s*"([^"]+)"`)
						}
						matches := bindAddrRegex.FindStringSubmatch(result)
						if len(matches) < 2 {
							t.Fatalf("Could not find bind-address in result")
						}

						if matches[1] != bindTest.expectedAddr {
							t.Errorf("Expected bind address %s, got %s", bindTest.expectedAddr, matches[1])
						}
					})
				}
			})

			t.Run("ETCD Servers Configuration", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"etcdServers": []string{
						"https://etcd1.example.com:2379",
						"https://etcd2.example.com:2379",
					},
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "etcd-servers") {
					t.Error("Expected etcd-servers not found")
				}
				if !strings.Contains(result, "https://127.0.0.1:2379") {
					t.Error("Expected default etcd server not found")
				}
				if !strings.Contains(result, "https://etcd1.example.com:2379") {
					t.Error("Expected additional etcd server 1 not found")
				}
				if !strings.Contains(result, "https://etcd2.example.com:2379") {
					t.Error("Expected additional etcd server 2 not found")
				}
			})

			t.Run("Secret Encryption", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["runType"] = "Runtime" // encryption is only added in runtime mode
				data["apiserver"] = map[string]interface{}{
					"secretEncryptionKey": "some-key",
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				// Both v1beta3 and v1beta4 should include encryption-provider-config
				if !strings.Contains(result, "encryption-provider-config") {
					t.Error("Expected encryption provider config not found")
				}
			})

			t.Run("Certificate SANs", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"certSANs": []string{
						"api.example.com",
						"192.168.1.100",
					},
				}

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				if !strings.Contains(result, "certSANs") {
					t.Error("Expected certSANs not found")
				}
				if !strings.Contains(result, "api.example.com") {
					t.Error("Expected SAN hostname not found")
				}
				if !strings.Contains(result, "192.168.1.100") {
					t.Error("Expected SAN IP not found")
				}
			})
		})
	}
}

func testClusterTypes(t *testing.T) {
	tests := []struct {
		name        string
		clusterType string
		expectCloud bool
	}{
		{
			name:        "Cloud cluster type",
			clusterType: "Cloud",
			expectCloud: true,
		},
		{
			name:        "Static cluster type",
			clusterType: "Static",
			expectCloud: false,
		},
	}

	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s (v%s)", tt.name, version), func(t *testing.T) {
				data := getBaseTemplateData(version)
				data["clusterConfiguration"].(map[string]interface{})["clusterType"] = tt.clusterType

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				cloudProviderFound := strings.Contains(result, "cloud-provider")
				if tt.expectCloud && !cloudProviderFound {
					t.Error("Expected cloud-provider configuration not found")
				}
				if !tt.expectCloud && cloudProviderFound {
					t.Error("Unexpected cloud-provider configuration found")
				}
			})
		}
	}
}

func testRunTypes(t *testing.T) {
	tests := []struct {
		name                   string
		runType                string
		expectAdmissionPlugins bool
		expectKubeletCA        bool
		expectSchedulerConfig  bool
	}{
		{
			name:                   "ClusterBootstrap run type",
			runType:                "ClusterBootstrap",
			expectAdmissionPlugins: false,
			expectKubeletCA:        false,
			expectSchedulerConfig:  false,
		},
		{
			name:                   "Runtime run type",
			runType:                "Runtime",
			expectAdmissionPlugins: true,
			expectKubeletCA:        true,
			expectSchedulerConfig:  true,
		},
	}

	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s (v%s)", tt.name, version), func(t *testing.T) {
				data := getBaseTemplateData(version)
				data["runType"] = tt.runType

				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config: %v", err)
				}

				admissionPluginsFound := strings.Contains(result, "enable-admission-plugins")
				kubeletCAFound := strings.Contains(result, "kubelet-certificate-authority")
				schedulerConfigFound := strings.Contains(result, "scheduler-config.yaml")

				if tt.expectAdmissionPlugins && !admissionPluginsFound {
					t.Error("Expected admission plugins configuration not found")
				}
				if !tt.expectAdmissionPlugins && admissionPluginsFound {
					t.Error("Unexpected admission plugins configuration found")
				}

				if tt.expectKubeletCA && !kubeletCAFound {
					t.Error("Expected kubelet CA configuration not found")
				}
				if !tt.expectKubeletCA && kubeletCAFound {
					t.Error("Unexpected kubelet CA configuration found")
				}

				if tt.expectSchedulerConfig && !schedulerConfigFound {
					t.Error("Expected scheduler config not found")
				}
				if !tt.expectSchedulerConfig && schedulerConfigFound {
					t.Error("Unexpected scheduler config found")
				}
			})
		}
	}
}

func testServiceAccountConfiguration(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Default Service Account (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			expectedIssuer := "https://kubernetes.default.svc.cluster.local"
			if !strings.Contains(result, expectedIssuer) {
				t.Errorf("Expected default issuer %s not found", expectedIssuer)
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Custom Service Account Issuer (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"serviceAccount": map[string]interface{}{
					"issuer": "https://custom.issuer.com",
				},
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "https://custom.issuer.com") {
				t.Error("Expected custom issuer not found")
			}
			if !strings.Contains(result, "https://custom.issuer.com/openid/v1/jwks") {
				t.Error("Expected custom JWKS URI not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Additional API Issuers (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"serviceAccount": map[string]interface{}{
					"issuer": "https://primary.issuer.com",
					"additionalAPIIssuers": []string{
						"https://additional1.issuer.com",
						"https://additional2.issuer.com",
					},
				},
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			var apiAudiencesRegex *regexp.Regexp
			if strings.Contains(result, "v1beta3") {
				apiAudiencesRegex = regexp.MustCompile(`api-audiences:\s*([^\s]+)`)
			} else {
				apiAudiencesRegex = regexp.MustCompile(`- name: api-audiences\s+value:\s*([^\s]+)`)
			}
			matches := apiAudiencesRegex.FindStringSubmatch(result)
			if len(matches) < 2 {
				t.Fatalf("Could not find api-audiences in result")
			}

			audiences := matches[1]
			if !strings.Contains(audiences, "https://primary.issuer.com") {
				t.Error("Expected primary issuer in audiences not found")
			}
			if !strings.Contains(audiences, "https://additional1.issuer.com") {
				t.Error("Expected additional issuer 1 in audiences not found")
			}
			if !strings.Contains(audiences, "https://additional2.issuer.com") {
				t.Error("Expected additional issuer 2 in audiences not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Additional API Audiences (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"serviceAccount": map[string]interface{}{
					"issuer": "https://primary.issuer.com",
					"additionalAPIAudiences": []string{
						"https://audience1.com",
						"https://audience2.com",
					},
				},
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			var apiAudiencesRegex *regexp.Regexp
			if strings.Contains(result, "v1beta3") {
				apiAudiencesRegex = regexp.MustCompile(`api-audiences:\s*([^\s]+)`)
			} else {
				apiAudiencesRegex = regexp.MustCompile(`- name: api-audiences\s+value:\s*([^\s]+)`)
			}
			matches := apiAudiencesRegex.FindStringSubmatch(result)
			if len(matches) < 2 {
				t.Fatalf("Could not find api-audiences in result")
			}

			audiences := matches[1]
			if !strings.Contains(audiences, "https://audience1.com") {
				t.Error("Expected additional audience 1 not found")
			}
			if !strings.Contains(audiences, "https://audience2.com") {
				t.Error("Expected additional audience 2 not found")
			}
		})
	}
}

func testETCDConfiguration(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("No ETCD Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if strings.Contains(result, "etcd:") && strings.Contains(result, "local:") {
				t.Error("Unexpected etcd configuration found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Existing Cluster ETCD (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["etcd"] = map[string]interface{}{
				"existingCluster": true,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "etcd:") {
				t.Error("Expected etcd configuration not found")
			}
			if !strings.Contains(result, "initial-cluster-state") {
				t.Error("Expected initial-cluster-state not found")
			}
			if !strings.Contains(result, "existing") {
				t.Error("Expected existing cluster state not found")
			}
			if !strings.Contains(result, "experimental-initial-corrupt-check") {
				t.Error("Expected corrupt check not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("ETCD with Quota (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["etcd"] = map[string]interface{}{
				"existingCluster":   true,
				"quotaBackendBytes": "8589934592",
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "quota-backend-bytes") {
				t.Error("Expected quota-backend-bytes not found")
			}
			if !strings.Contains(result, "8589934592") {
				t.Error("Expected quota value not found")
			}
			if !strings.Contains(result, "metrics") {
				t.Error("Expected metrics configuration not found")
			}
			if !strings.Contains(result, "extensive") {
				t.Error("Expected extensive metrics not found")
			}
		})
	}
}

func testOptionalArguments(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Node Monitor Arguments (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["arguments"] = map[string]interface{}{
				"nodeMonitorPeriod":      30,
				"nodeMonitorGracePeriod": 60,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "node-monitor-period") {
				t.Error("Expected node-monitor-period not found")
			}
			if !strings.Contains(result, "30s") {
				t.Error("Expected node monitor period value not found")
			}
			if !strings.Contains(result, "node-monitor-grace-period") {
				t.Error("Expected node-monitor-grace-period not found")
			}
			if !strings.Contains(result, "60s") {
				t.Error("Expected node monitor grace period value not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Pod Eviction Timeout (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["arguments"] = map[string]interface{}{
				"podEvictionTimeout":                  120,
				"defaultUnreachableTolerationSeconds": 300,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "default-not-ready-toleration-seconds") {
				t.Error("Expected default-not-ready-toleration-seconds not found")
			}
			if !strings.Contains(result, "120") {
				t.Error("Expected pod eviction timeout value not found")
			}
			if !strings.Contains(result, "default-unreachable-toleration-seconds") {
				t.Error("Expected default-unreachable-toleration-seconds not found")
			}
			if !strings.Contains(result, "300") {
				t.Error("Expected unreachable toleration value not found")
			}
		})
	}
}

func testPatchesRendering(t *testing.T) {
	t.Run("All Patches Render Successfully", func(t *testing.T) {
		versions := []string{"1.30", "1.31", "1.32"}

		for _, version := range versions {
			t.Run("Version "+version, func(t *testing.T) {
				data := getBaseTemplateData(version)
				data["apiserver"] = map[string]interface{}{
					"webhookURL":        "https://webhook.example.com",
					"oidcIssuerURL":     "https://oidc.example.com",
					"oidcIssuerAddress": "192.168.1.100",
				}
				data["images"] = map[string]interface{}{
					"controlPlaneManager": map[string]interface{}{
						"kubeApiserverHealthcheck": "sha256:abcd1234",
						"kubeApiserver131":         "sha256:efgh5678",
						"kubeApiserver130":         "sha256:ijkl9012",
						"kubeApiserver129":         "sha256:mnop3456",
					},
				}
				data["registry"] = map[string]interface{}{
					"address": "registry.example.com",
					"path":    "/deckhouse",
				}
				data["resourcesRequestsMilliCpuControlPlane"] = 1000
				data["resourcesRequestsMemoryControlPlane"] = 1073741824

				patches, err := renderKubeadmPatches(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm patches: %v", err)
				}

				expectedPatches := []string{
					"etcd.yaml",
					"kube-apiserver.yaml",
					"kube-controller-manager.yaml",
					"kube-scheduler.yaml",
				}

				for _, patchName := range expectedPatches {
					if _, exists := patches[patchName]; !exists {
						t.Errorf("Expected patch %s not found in rendered patches", patchName)
					}
				}
			})
		}
	})
}

func testEdgeCases(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Complex Configuration Combination (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["runType"] = "Runtime"
			data["nodeIP"] = "192.168.1.100"
			data["apiserver"] = map[string]interface{}{
				"webhookURL":          "https://webhook.example.com",
				"authnWebhookURL":     "https://authn.example.com",
				"auditWebhookURL":     "https://audit.example.com",
				"oidcIssuerURL":       "https://oidc.example.com",
				"secretEncryptionKey": "test-key",
				"bindToWildcard":      false,
				"etcdServers": []string{
					"https://etcd1.example.com:2379",
				},
				"certSANs": []string{
					"api.example.com",
				},
				"admissionPlugins": []string{
					"CustomAdmissionPlugin",
				},
				"serviceAccount": map[string]interface{}{
					"issuer": "https://custom.issuer.com",
					"additionalAPIIssuers": []string{
						"https://additional.issuer.com",
					},
				},
				"auditPolicy": "complex-policy",
				"auditLog": map[string]interface{}{
					"output": "File",
					"path":   "/var/log/audit",
				},
			}
			data["etcd"] = map[string]interface{}{
				"existingCluster":   true,
				"quotaBackendBytes": "8589934592",
			}
			data["arguments"] = map[string]interface{}{
				"nodeMonitorPeriod":                   45,
				"nodeMonitorGracePeriod":              90,
				"podEvictionTimeout":                  180,
				"defaultUnreachableTolerationSeconds": 360,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render complex kubeadm config: %v", err)
			}

			expectedStrings := []string{
				"Node,Webhook,RBAC",
				"authentication-token-webhook-config-file",
				"audit-webhook-config-file",
				"authentication-config",
				"api.example.com",
				"CustomAdmissionPlugin",
				"https://custom.issuer.com",
				"https://additional.issuer.com",
				"8589934592",
				"45s",
				"180",
			}

			for _, expected := range expectedStrings {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected string %s not found in result", expected)
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Minimal Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render minimal kubeadm config: %v", err)
			}

			if len(result) == 0 {
				t.Error("Empty result for minimal configuration")
			}

			var requiredStrings []string
			if strings.Contains(result, "v1beta3") {
				requiredStrings = []string{
					"apiVersion: kubeadm.k8s.io/v1beta3",
					"kind: ClusterConfiguration",
					fmt.Sprintf("kubernetesVersion: %s.1", version),
					"service-account-issuer",
					"feature-gates",
				}
			} else {
				requiredStrings = []string{
					"apiVersion: kubeadm.k8s.io/v1beta4",
					"kind: ClusterConfiguration",
					fmt.Sprintf("kubernetesVersion: %s.1", version),
					"service-account-issuer",
					"feature-gates",
				}
			}

			for _, required := range requiredStrings {
				if !strings.Contains(result, required) {
					t.Errorf("Required string %s not found in minimal config", required)
				}
			}
		})
	}

	t.Run("Version Boundary Cases", func(t *testing.T) {
		testCases := []struct {
			version     string
			expectBeta3 bool
		}{
			{"1.30", true},
			{"1.31", false},
			{"1.32", false},
		}

		for _, tc := range testCases {
			t.Run("Version "+tc.version, func(t *testing.T) {
				data := getBaseTemplateData(tc.version)
				result, err := renderKubeadmConfig(data)
				if err != nil {
					t.Fatalf("Failed to render kubeadm config for version %s: %v", tc.version, err)
				}

				containsBeta3 := strings.Contains(result, "kubeadm.k8s.io/v1beta3")
				containsBeta4 := strings.Contains(result, "kubeadm.k8s.io/v1beta4")

				if tc.expectBeta3 {
					if !containsBeta3 {
						t.Errorf("Expected v1beta3 for version %s", tc.version)
					}
					if containsBeta4 {
						t.Errorf("Unexpected v1beta4 for version %s", tc.version)
					}
				} else {
					if containsBeta3 {
						t.Errorf("Unexpected v1beta3 for version %s", tc.version)
					}
					if !containsBeta4 {
						t.Errorf("Expected v1beta4 for version %s", tc.version)
					}
				}
			})
		}
	})
}

func getBaseTemplateData(k8sVersion string) map[string]interface{} {
	return map[string]interface{}{
		"nodeIP":  "127.0.0.1",
		"runType": "ClusterBootstrap",
		"clusterConfiguration": map[string]interface{}{
			"kubernetesVersion":       k8sVersion,
			"clusterType":             "Static",
			"serviceSubnetCIDR":       "10.222.0.0/16",
			"podSubnetCIDR":           "10.111.0.0/16",
			"podSubnetNodeCIDRPrefix": "24",
			"clusterDomain":           "cluster.local",
			"encryptionAlgorithm":     "ECDSA-P256",
		},
		"k8s": map[string]interface{}{
			k8sVersion: map[string]interface{}{
				"patch": 1,
			},
		},
		"extraArgs": map[string]interface{}{},
		"registry": map[string]interface{}{
			"address": "registry.deckhouse.io",
			"path":    "/deckhouse/ce",
		},
		"images": map[string]interface{}{},
	}
}

func renderKubeadmConfig(data map[string]interface{}) (string, error) {
	cc := data["clusterConfiguration"].(map[string]interface{})
	k8sVer := cc["kubernetesVersion"].(string)
	kubeadmVersion, err := GetKubeadmVersion(k8sVer)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join("/deckhouse/candi/control-plane-kubeadm", kubeadmVersion, "config.yaml.tpl")

	tplContent, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	configResult, err := RenderTemplate("config.yaml.tpl", tplContent, data)
	if err != nil {
		return "", err
	}

	return configResult.Content.String(), nil
}

func renderKubeadmPatches(data map[string]interface{}) (map[string]string, error) {
	cc := data["clusterConfiguration"].(map[string]interface{})
	k8sVer := cc["kubernetesVersion"].(string)
	kubeadmVersion, err := GetKubeadmVersion(k8sVer)
	if err != nil {
		return nil, err
	}

	patchesPath := filepath.Join("/deckhouse/candi/control-plane-kubeadm", kubeadmVersion, "patches")

	patches := make(map[string]string)
	patchFiles := []string{
		"etcd.yaml.tpl",
		"kube-apiserver.yaml.tpl",
		"kube-controller-manager.yaml.tpl",
		"kube-scheduler.yaml.tpl",
	}

	for _, patchFile := range patchFiles {
		patchPath := filepath.Join(patchesPath, patchFile)

		tplContent, err := os.ReadFile(patchPath)
		if err != nil {
			return nil, err
		}

		patchResult, err := RenderTemplate(patchFile, tplContent, data)
		if err != nil {
			return nil, err
		}

		patches[strings.TrimSuffix(patchFile, ".tpl")] = patchResult.Content.String()
	}

	return patches, nil
}

func testMissingCoverage(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Runtime Config Version Condition (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "runtime-config") {
				t.Error("Expected runtime-config for Kubernetes >= 1.28 not found")
			}
			if !strings.Contains(result, "admissionregistration.k8s.io/v1beta1=true") {
				t.Error("Expected runtime-config value not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("No NodeIP Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			delete(data, "nodeIP")

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "kind: ClusterConfiguration") {
				t.Error("Expected ClusterConfiguration not found")
			}

			if strings.Contains(result, "advertiseAddress:") {
				t.Error("Unexpected advertiseAddress found when nodeIP is not set")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Patches Without NodeIP (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			delete(data, "nodeIP")
			data["images"] = map[string]interface{}{
				"controlPlaneManager": map[string]interface{}{
					"kubeApiserverHealthcheck": "sha256:abcd1234",
				},
			}
			data["registry"] = map[string]interface{}{
				"address": "registry.example.com",
				"path":    "/deckhouse",
			}

			patches, err := renderKubeadmPatches(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm patches: %v", err)
			}

			apiserverPatch, exists := patches["kube-apiserver.yaml"]
			if !exists {
				t.Error("Expected kube-apiserver patch not found")
			}

			if strings.Contains(apiserverPatch, "host: ") {
				t.Error("Unexpected host configuration found when nodeIP is not set")
			}
		})
	}

	t.Run("V1beta3 Additional Conditions", func(t *testing.T) {
		// Test v1beta3 specific conditions
		data := getBaseTemplateData("1.30")
		data["runType"] = "Runtime"

		result, err := renderKubeadmConfig(data)
		if err != nil {
			t.Fatalf("Failed to render v1beta3 kubeadm config: %v", err)
		}

		// v1beta3 should have runtime-config without version condition
		if !strings.Contains(result, "runtime-config:") {
			t.Error("Expected runtime-config in v1beta3 not found")
		}

		// v1beta3 uses different format (map syntax)
		if !strings.Contains(result, `runtime-config: "admissionregistration.k8s.io/v1beta1=true`) {
			t.Error("Expected v1beta3 runtime-config format not found")
		}
	})

	for _, version := range versions {
		t.Run(fmt.Sprintf("Service Account Edge Cases (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"serviceAccount": map[string]interface{}{
					"additionalAPIIssuers": []string{
						"https://external.issuer.com",
					},
				},
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if !strings.Contains(result, "https://kubernetes.default.svc.cluster.local") {
				t.Error("Expected default service account issuer not found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("ETCD Without Existing Cluster (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["etcd"] = map[string]interface{}{
				"existingCluster": false,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if strings.Contains(result, "initial-cluster-state: existing") {
				t.Error("Unexpected etcd existing cluster configuration found")
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Audit Volume Mount Edge Cases (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"auditPolicy": "some-policy",
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if strings.Contains(result, "kube-audit-log") {
				t.Error("Unexpected audit log volume mount found")
			}
		})
	}

	t.Run("Feature Gates Version Boundaries", func(t *testing.T) {
		// Test exactly version 1.30 boundary
		data := getBaseTemplateData("1.30")
		result, err := renderKubeadmConfig(data)
		if err != nil {
			t.Fatalf("Failed to render kubeadm config: %v", err)
		}

		// Should NOT include legacy feature gates for 1.30
		featureGatesRegex := regexp.MustCompile(`feature-gates:\s*"([^"]+)"`)
		matches := featureGatesRegex.FindStringSubmatch(result)
		if len(matches) >= 2 {
			featureGates := matches[1]
			if strings.Contains(featureGates, "ValidatingAdmissionPolicy=true") {
				t.Error("Unexpected legacy feature gate found for Kubernetes 1.30")
			}
		}
	})

	for _, version := range versions {
		t.Run(fmt.Sprintf("API Version Specific Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["runType"] = "Runtime"
			data["apiserver"] = map[string]interface{}{
				"bindToWildcard": true,
			}

			result, err := renderKubeadmConfig(data)
			if err != nil {
				t.Fatalf("Failed to render kubeadm config: %v", err)
			}

			if strings.Contains(result, "v1beta3") {
				if !strings.Contains(result, "apiVersion: kubeadm.k8s.io/v1beta3") {
					t.Error("Expected v1beta3 API version not found")
				}
				if !strings.Contains(result, `bind-address: "0.0.0.0"`) {
					t.Error("Expected v1beta3 bind-address format not found")
				}
			} else {
				if !strings.Contains(result, "apiVersion: kubeadm.k8s.io/v1beta4") {
					t.Error("Expected v1beta4 API version not found")
				}
				if !strings.Contains(result, "- name: bind-address") {
					t.Error("Expected v1beta4 name/value format for bind-address not found")
				}
				if !strings.Contains(result, `value: "0.0.0.0"`) {
					t.Error("Expected v1beta4 bind-address value not found")
				}
				if !strings.Contains(result, "- name: feature-gates") {
					t.Error("Expected v1beta4 name/value format for feature-gates not found")
				}
				if !strings.Contains(result, "- name: runtime-config") {
					t.Error("Expected v1beta4 name/value format for runtime-config not found")
				}
				if !strings.Contains(result, "value: admissionregistration.k8s.io/v1beta1=true") {
					t.Error("Expected v1beta4 runtime-config value not found")
				}
			}
		})
	}
}
