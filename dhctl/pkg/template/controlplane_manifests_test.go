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

package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestControlplaneRendering(t *testing.T) {
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
	t.Run("Full Manifests Rendering", testManifestsRendering)
}

func testVersionSelection(t *testing.T) {
	tests := []struct {
		name         string
		k8sVersion   string
		expectedAPI  string
		expectedKind string
	}{
		{
			name:         "Kubernetes 1.31 should generate pod manifests",
			k8sVersion:   "1.31",
			expectedAPI:  "apiVersion: v1",
			expectedKind: "kind: Pod",
		},
		{
			name:         "Kubernetes 1.32 should generate pod manifests",
			k8sVersion:   "1.32",
			expectedAPI:  "apiVersion: v1",
			expectedKind: "kind: Pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := getBaseTemplateData(tt.k8sVersion)
			manifests, err := renderFullManifests(data)
			if err != nil {
				t.Fatalf("Failed to render pod manifests: %v", err)
			}

			for name, manifest := range manifests {
				if !strings.Contains(manifest, tt.expectedAPI) {
					t.Errorf("Expected API version %s not found in manifest %s", tt.expectedAPI, name)
				}

				if !strings.Contains(manifest, tt.expectedKind) {
					t.Errorf("Expected kind %s not found in manifest %s", tt.expectedKind, name)
				}
			}
		})
	}
}

func testManifestsRendering(t *testing.T) {
	t.Run("All Control Plane Pod Manifests Render Successfully", func(t *testing.T) {
		versions := []string{"1.31", "1.32"}

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
						"etcd":                     "sha256:62c84f",
						"kubeApiserver131":         "sha256:5db2b9",
						"kubeApiserver132":         "sha256:b4b2b5",
						"kubeControllerManager131": "sha256:acb28d",
						"kubeControllerManager132": "sha256:177438",
						"kubeScheduler131":         "sha256:2e366b",
						"kubeScheduler132":         "sha256:268cf6",
					},
				}
				data["registry"] = map[string]interface{}{
					"address": "registry.example.com",
					"path":    "/deckhouse",
				}
				data["resourcesRequestsMilliCpuControlPlane"] = 1000
				data["resourcesRequestsMemoryControlPlane"] = 1073741824

				manifests, err := renderFullManifests(data)
				if err != nil {
					t.Fatalf("Failed to render control plane pod manifests: %v", err)
				}

				expectedManifests := []string{
					"etcd.yaml",
					"kube-apiserver.yaml",
					"kube-controller-manager.yaml",
					"kube-scheduler.yaml",
				}

				for _, manifestName := range expectedManifests {
					if _, exists := manifests[manifestName]; !exists {
						t.Errorf("Expected pod manifest %s not found in rendered manifests", manifestName)
					}
				}
			})
		}
	})
}

func testFeatureGates(t *testing.T) {
	tests := []struct {
		name             string
		k8sVersion       string
		expectedFeatures []string
	}{
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
			result, err := renderFullManifests(data, "kube-apiserver", "kube-scheduler", "kube-controller-manager")
			if err != nil {
				t.Fatalf("Failed to render control plane pod manifests: %v", err)
			}

			featureGatesRegex := regexp.MustCompile(`--feature-gates=([^\n]+)\n`)

			for name, manifest := range result {
				matches := featureGatesRegex.FindStringSubmatch(manifest)
				if len(matches) < 2 {
					t.Fatalf("Could not find feature-gates in manifest %s", name)
				}

				featureGates := matches[1]
				for _, expected := range tt.expectedFeatures {
					if !strings.Contains(featureGates, expected) {
						t.Errorf("Expected feature gate %s not found in: %s of manifest %s", expected, featureGates, name)
					}
				}

				if name == "kube-apiserver.yaml" {
					if !strings.Contains(featureGates, "CRDSensitiveData=true") {
						t.Errorf("Expected CRDSensitiveData=true in kube-apiserver, got: %s", featureGates)
					}
				} else if strings.Contains(featureGates, "CRDSensitiveData=true") {
					t.Errorf("CRDSensitiveData must be kube-apiserver-only, found in %s: %s", name, featureGates)
				}

				if tt.k8sVersion >= "1.30" {
					unexpectedFeatures := []string{
						"ValidatingAdmissionPolicy=true",
						"AdmissionWebhookMatchConditions=true",
						"StructuredAuthenticationConfiguration=true",
					}
					for _, unexpected := range unexpectedFeatures {
						if strings.Contains(featureGates, unexpected) {
							t.Errorf("Unexpected feature gate %s found in: %s of manifest %s", unexpected, featureGates, name)
						}
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
			name:       "authentication configuration",
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

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane configs: %v", err)
				}
				for name, manifest := range result {
					// When webhookURL is configured, we always use structured authorization config (authorization-config).
					if !strings.Contains(manifest, "--authorization-config") {
						t.Errorf("Expected authorization-config not found in %s", name)
					}
					if strings.Contains(manifest, "--authorization-mode") {
						t.Errorf("Unexpected authorization-mode found in %s", name)
					}
					for _, expected := range []string{
						"seccompProfile:",
						"failureThreshold: 8",
						"initialDelaySeconds: 10",
						"timeoutSeconds: 15",
						"failureThreshold: 24",
					} {
						if !strings.Contains(manifest, expected) {
							t.Errorf("Expected static pod default %s not found in %s", expected, name)
						}
					}
				}
			})

			t.Run("Authentication Webhook", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"authnWebhookURL":      "https://authn.example.com",
					"authnWebhookCacheTTL": "10m",
				}

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					if !strings.Contains(manifest, "--authentication-token-webhook-config-file") {
						t.Errorf("Expected authentication webhook config file not found in %s", name)
					}
					if !strings.Contains(manifest, "--authentication-token-webhook-cache-ttl") {
						t.Errorf("Expected authentication webhook cache TTL not found in %s", name)
					}
					if !strings.Contains(manifest, "=10m") {
						t.Errorf("Expected cache TTL value not found in %s", name)
					}
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

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					if !strings.Contains(manifest, "--audit-policy-file") {
						t.Errorf("Expected audit policy file not found in %s", name)
					}
					if !strings.Contains(manifest, "--audit-log-path") {
						t.Errorf("Expected audit log path not found in %s", name)
					}
					if !strings.Contains(manifest, "=/var/log/kube-audit/audit.log") {
						t.Errorf("Expected audit log file path not found in %s", name)
					}
					if !strings.Contains(manifest, "audit-log-truncate-enabled") {
						t.Errorf("Expected audit log truncate not found in %s", name)
					}
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

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					foundStdout := strings.Contains(manifest, "--audit-log-path=-")
					if !foundStdout {
						t.Errorf("Expected stdout audit log path not found in %s", name)
					}
				}
			})

			t.Run("Audit Webhook", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"auditWebhookURL": "https://audit.example.com",
				}

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					if !strings.Contains(manifest, "audit-webhook-config-file") {
						t.Errorf("Expected audit webhook config file not found in %s", name)
					}
				}
			})

			t.Run("OIDC Configuration", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["apiserver"] = map[string]interface{}{
					"oidcIssuerURL": "https://oidc.example.com",
				}

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					if !strings.Contains(manifest, "authentication-config") {
						t.Errorf("Expected authentication config not found in %s", name)
					}
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

						result, err := renderFullManifests(data, "kube-apiserver")
						if err != nil {
							t.Fatalf("Failed to render control-plane config: %v", err)
						}
						for name, manifest := range result {
							bindAddrRegex := regexp.MustCompile(`- --bind-address=([^\n]+)\n`)
							matches := bindAddrRegex.FindStringSubmatch(manifest)
							if len(matches) < 2 {
								t.Fatalf("Could not find bind-address in result %s", name)
							}

							if matches[1] != bindTest.expectedAddr {
								t.Errorf("Expected bind address %s, got %s in %s", bindTest.expectedAddr, matches[1], name)
							}
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

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					if !strings.Contains(manifest, "etcd-servers") {
						t.Errorf("Expected etcd-servers not found in %s", name)
					}
					if !strings.Contains(manifest, "https://127.0.0.1:2379") {
						t.Errorf("Expected default etcd server not found in %s", name)
					}
					if !strings.Contains(manifest, "https://etcd1.example.com:2379") {
						t.Errorf("Expected additional etcd server 1 not found in %s", name)
					}
					if !strings.Contains(manifest, "https://etcd2.example.com:2379") {
						t.Errorf("Expected additional etcd server 2 not found in %s", name)
					}
				}
			})

			t.Run("Secret Encryption", func(t *testing.T) {
				data := getBaseTemplateData(tt.k8sVersion)
				data["runType"] = "Runtime" // encryption is only added in runtime mode
				data["apiserver"] = map[string]interface{}{
					"secretEncryptionKey": "some-key",
				}

				result, err := renderFullManifests(data, "kube-apiserver")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					// kubeadm configuration should include encryption-provider-config
					if !strings.Contains(manifest, "encryption-provider-config") {
						t.Errorf("Expected encryption provider config not found in %s", name)
					}
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

				result, err := renderFullManifests(data, "kube-controller-manager")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					cloudProviderFound := strings.Contains(manifest, "--cloud-provider=external")
					if tt.expectCloud && !cloudProviderFound {
						t.Errorf("Expected cloud-provider configuration not found in %s", name)
					}
					if !tt.expectCloud && cloudProviderFound {
						t.Errorf("Unexpected cloud-provider configuration found in %s", name)
					}
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

				result, err := renderFullManifests(data, "kube-apiserver", "kube-scheduler")
				if err != nil {
					t.Fatalf("Failed to render control-plane config: %v", err)
				}
				for name, manifest := range result {
					switch name {
					case "kube-apiserver.yaml":
						admissionPluginsFound := strings.Contains(manifest, "--enable-admission-plugins")
						kubeletCAFound := strings.Contains(manifest, "--kubelet-certificate-authority")
						if tt.expectAdmissionPlugins && !admissionPluginsFound {
							t.Errorf("Expected admission plugins configuration not in %s", name)
						}
						if !tt.expectAdmissionPlugins && admissionPluginsFound {
							t.Errorf("Unexpected admission plugins configuration in %s", name)
						}

						if tt.expectKubeletCA && !kubeletCAFound {
							t.Errorf("Expected kubelet CA configuration not in %s", name)
						}
						if !tt.expectKubeletCA && kubeletCAFound {
							t.Errorf("Unexpected kubelet CA configuration in %s", name)
						}
					case "kube-scheduler.yaml":
						schedulerConfigFound := strings.Contains(manifest, "scheduler-config.yaml")
						if tt.expectSchedulerConfig && !schedulerConfigFound {
							t.Errorf("Expected scheduler config not in %s", name)
						}
						if !tt.expectSchedulerConfig && schedulerConfigFound {
							t.Errorf("Unexpected scheduler config in %s", name)
						}
					}
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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for name, manifest := range result {
				expectedIssuer := "https://kubernetes.default.svc.cluster.local"
				if !strings.Contains(manifest, expectedIssuer) {
					t.Errorf("Expected default issuer %s not found in %s", expectedIssuer, name)
				}
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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "https://custom.issuer.com") {
					t.Error("Expected custom issuer not found")
				}
				if !strings.Contains(manifest, "https://custom.issuer.com/openid/v1/jwks") {
					t.Error("Expected custom JWKS URI not found")
				}
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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				apiAudiencesRegex := regexp.MustCompile(`--api-audiences=([^\s]+)\s`)
				matches := apiAudiencesRegex.FindStringSubmatch(manifest)
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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				apiAudiencesRegex := regexp.MustCompile(`--api-audiences=([^\s]+)\s`)
				matches := apiAudiencesRegex.FindStringSubmatch(manifest)
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
			}
		})
	}
}

func testETCDConfiguration(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("No ETCD Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)

			result, err := renderFullManifests(data, "etcd")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if strings.Contains(manifest, "initial-cluster-state=existing") {
					t.Error("Unexpected existing etcd cluster configuration found")
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Existing Cluster ETCD (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["etcd"] = map[string]interface{}{
				"existingCluster": true,
			}

			result, err := renderFullManifests(data, "etcd")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "initial-cluster-state=existing") {
					t.Error("Expected initial-cluster-state not found")
				}
				if !strings.Contains(manifest, "InitialCorruptCheck=true") {
					t.Error("Expected corrupt check not found")
				}
				if !strings.Contains(manifest, "--metrics=extensive") {
					t.Error("Expected metrics configuration not found")
				}
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

			result, err := renderFullManifests(data, "etcd")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "--quota-backend-bytes=8589934592") {
					t.Error("Expected quota-backend-bytes not found")
				}
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

			result, err := renderFullManifests(data, "kube-controller-manager")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "--node-monitor-period=30s") {
					t.Error("Expected node-monitor-period not found")
				}
				if !strings.Contains(manifest, "--node-monitor-grace-period=60s") {
					t.Error("Expected node-monitor-grace-period not found")
				}
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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "--default-not-ready-toleration-seconds=120") {
					t.Error("Expected default-not-ready-toleration-seconds not found")
				}
				if !strings.Contains(manifest, "--default-unreachable-toleration-seconds=300") {
					t.Error("Expected default-unreachable-toleration-seconds not found")
				}
			}
		})
	}
}

func testPatchesRendering(t *testing.T) {
	t.Run("All Patches Render Successfully", func(t *testing.T) {
		versions := []string{"1.31", "1.32"}

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

				manifests, err := renderFullManifests(data)
				if err != nil {
					t.Fatalf("Failed to render pod manifests: %v", err)
				}

				expectedManifests := []string{
					"etcd.yaml",
					"kube-apiserver.yaml",
					"kube-controller-manager.yaml",
					"kube-scheduler.yaml",
				}

				for _, manifestName := range expectedManifests {
					if _, exists := manifests[manifestName]; !exists {
						t.Errorf("Expected manifest %s not found in rendered manifests", manifestName)
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

			result, err := renderFullManifests(data, "kube-apiserver", "kube-controller-manager", "etcd")
			if err != nil {
				t.Fatalf("Failed to render complex kubeadm config: %v", err)
			}

			expectedStringsApiserver := []string{
				"authorization-config", // structured authorization config (instead of Node,Webhook,RBAC flags)
				"authentication-token-webhook-config-file",
				"audit-webhook-config-file",
				"authentication-config",
				"CustomAdmissionPlugin",
				"https://custom.issuer.com",
				"https://additional.issuer.com",
				"180",
			}
			expectedStringsControllerManager := []string{
				"45s",
			}
			expectedStringsEtcd := []string{
				"8589934592",
			}
			for name, manifest := range result {
				switch name {
				case "kube-apiserver.yaml":
					for _, expected := range expectedStringsApiserver {
						if !strings.Contains(manifest, expected) {
							t.Errorf("Expected string %s not found in result %s", expected, name)
						}
					}
				case "kube-controller-manager.yaml":
					for _, expected := range expectedStringsControllerManager {
						if !strings.Contains(manifest, expected) {
							t.Errorf("Expected string %s not found in result %s", expected, name)
						}
					}
				case "etcd.yaml":
					for _, expected := range expectedStringsEtcd {
						if !strings.Contains(manifest, expected) {
							t.Errorf("Expected string %s not found in result %s", expected, name)
						}
					}
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Minimal Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)

			manifests, err := renderFullManifests(data)
			if err != nil {
				t.Fatalf("Failed to render minimal kubeadm config: %v", err)
			}

			if len(manifests) == 0 {
				t.Error("Empty result for minimal configuration")
			}

			for name, manifest := range manifests {
				if len(manifest) == 0 {
					t.Errorf("Empty manifest for %s", name)
				}

				requiredStrings := []string{
					"apiVersion: v1",
					"kind: Pod",
				}

				for _, required := range requiredStrings {
					if !strings.Contains(manifest, required) {
						t.Errorf("Required string %s not found in minimal manifest %s", required, name)
					}
				}
			}
		})
	}
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

// Don't pass requestedManifests to get all manifests
func renderFullManifests(data map[string]interface{}, requestedManifests ...string) (map[string]string, error) {
	templatesPath := "/deckhouse/candi/control-plane"
	manifests := make(map[string]string)

	templateMap := map[string]string{
		"etcd":                    "etcd.yaml.tpl",
		"kube-apiserver":          "kube-apiserver.yaml.tpl",
		"kube-controller-manager": "kube-controller-manager.yaml.tpl",
		"kube-scheduler":          "kube-scheduler.yaml.tpl",
	}

	// If no specific manifests requested, use all from the map
	manifestsToRender := requestedManifests
	if len(requestedManifests) == 0 {
		for manifestName := range templateMap {
			manifestsToRender = append(manifestsToRender, manifestName)
		}
	}

	requestedSet := make(map[string]bool)
	for _, manifest := range manifestsToRender {
		requestedSet[manifest] = true
	}

	for manifestName, templateFile := range templateMap {
		if !requestedSet[manifestName] {
			continue
		}

		templatePath := filepath.Join(templatesPath, templateFile)
		tplContent, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, err
		}
		templateResult, err := RenderTemplate(templateFile, tplContent, data)
		if err != nil {
			return nil, err
		}
		manifests[strings.TrimSuffix(templateFile, ".tpl")] = templateResult.Content.String()
	}

	return manifests, nil
}

func testMissingCoverage(t *testing.T) {
	versions := []string{"1.31", "1.32"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Runtime Config Version Condition (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "runtime-config") {
					t.Error("Expected runtime-config for Kubernetes >= 1.28 not found")
				}
				if !strings.Contains(manifest, "admissionregistration.k8s.io/v1beta1=true") {
					t.Error("Expected runtime-config value not found")
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("No NodeIP Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			delete(data, "nodeIP")
			data["registry"] = map[string]interface{}{
				"address": "registry.example.com",
				"path":    "/deckhouse",
			}

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "--bind-address=0.0.0.0") {
					t.Error("Expected bind address any not found when nodeIP is not set")
				}

				if strings.Contains(manifest, "--advertise-address") {
					t.Error("Unexpected advertiseAddress found when nodeIP is not set")
				}
				if strings.Contains(manifest, "host: ") {
					t.Error("Unexpected host configuration found when nodeIP is not set")
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Manifests Without NodeIP (v%s)", version), func(t *testing.T) {
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

			manifests, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render pod manifests: %v", err)
			}

			apiserverManifest, exists := manifests["kube-apiserver.yaml"]
			if !exists {
				t.Error("Expected kube-apiserver manifest not found")
			}

			if strings.Contains(apiserverManifest, "host: ") {
				t.Error("Unexpected host configuration found when nodeIP is not set")
			}
		})
	}

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

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if !strings.Contains(manifest, "https://kubernetes.default.svc.cluster.local") {
					t.Error("Expected default service account issuer not found")
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("ETCD Without Existing Cluster (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["etcd"] = map[string]interface{}{
				"existingCluster": false,
			}

			result, err := renderFullManifests(data, "etcd")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if strings.Contains(manifest, "initial-cluster-state=existing") {
					t.Error("Unexpected etcd existing cluster configuration found")
				}
			}
		})
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("Audit Volume Mount Edge Cases (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["apiserver"] = map[string]interface{}{
				"auditPolicy": "some-policy",
			}

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for _, manifest := range result {
				if strings.Contains(manifest, "kube-audit-log") {
					t.Error("Unexpected audit log volume mount found")
				}
			}
		})
	}

	t.Run("Feature Gates Version Boundaries", func(t *testing.T) {
		// Test exactly version 1.31 boundary
		data := getBaseTemplateData("1.31")
		result, err := renderFullManifests(data, "kube-apiserver", "kube-controller-manager", "kube-scheduler")
		if err != nil {
			t.Fatalf("Failed to render control-plane config: %v", err)
		}

		featureGatesRegex := regexp.MustCompile(`--feature-gates=([^\n]+)\n`)
		for name, manifest := range result {
			matches := featureGatesRegex.FindStringSubmatch(manifest)
			if len(matches) >= 2 {
				featureGates := matches[1]
				if strings.Contains(featureGates, "ValidatingAdmissionPolicy=true") {
					t.Errorf("Unexpected legacy feature gate found for Kubernetes 1.31 in %s", name)
				}
				if !strings.Contains(featureGates, "AnonymousAuthConfigurableEndpoints=true") {
					t.Errorf("Expected feature gate not found for Kubernetes 1.31 in %s", name)
				}
				if name == "kube-apiserver.yaml" && !strings.Contains(featureGates, "CRDSensitiveData=true") {
					t.Errorf("Expected CRDSensitiveData=true for Kubernetes 1.31 in %s", name)
				}
			}
		}
	})

	for _, version := range versions {
		t.Run(fmt.Sprintf("Bind Address Configuration (v%s)", version), func(t *testing.T) {
			data := getBaseTemplateData(version)
			data["runType"] = "Runtime"
			data["apiserver"] = map[string]interface{}{
				"bindToWildcard": true,
			}

			result, err := renderFullManifests(data, "kube-apiserver")
			if err != nil {
				t.Fatalf("Failed to render control-plane config: %v", err)
			}
			for name, manifest := range result {
				if !strings.Contains(manifest, "apiVersion: v1") {
					t.Errorf("Expected v1 API version not found in %s", name)
				}
				if !strings.Contains(manifest, "kind: Pod") {
					t.Errorf("Expected Pod kind not found in %s", name)
				}
				if !strings.Contains(manifest, "--bind-address=0.0.0.0") {
					t.Errorf("Expected bind-address=0.0.0.0 not found in %s", name)
				}
				if !strings.Contains(manifest, "--feature-gates=") {
					t.Errorf("Expected feature-gates flag not found in %s", name)
				}
				if !strings.Contains(manifest, "--runtime-config=") {
					t.Errorf("Expected runtime-config flag not found in %s", name)
				}
				if !strings.Contains(manifest, "admissionregistration.k8s.io/v1beta1=true") {
					t.Errorf("Expected runtime-config value not found in %s", name)
				}
			}
		})
	}
}
