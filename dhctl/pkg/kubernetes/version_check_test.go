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

package kubernetes

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "version with v prefix",
			input:    "v1.73.0",
			expected: "1.73.0",
		},
		{
			name:     "version without v prefix",
			input:    "1.73.0",
			expected: "1.73.0",
		},
		{
			name:     "version with v prefix and whitespace",
			input:    " v1.73.0 ",
			expected: "1.73.0",
		},
		{
			name:     "version without v prefix and whitespace",
			input:    " 1.73.0 ",
			expected: "1.73.0",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only v",
			input:    "v",
			expected: "",
		},
		{
			name:     "version with multiple v",
			input:    "vv1.73.0",
			expected: "v1.73.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVersion(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image with tag",
			image:    "registry.deckhouse.io/deckhouse/ce:v1.73.0",
			expected: "v1.73.0",
		},
		{
			name:     "image with digest only",
			image:    "registry.deckhouse.io/deckhouse/ce@sha256:abc123def456",
			expected: "",
		},
		{
			name:     "image with tag and digest",
			image:    "registry.deckhouse.io/deckhouse/ce:v1.73.0@sha256:abc123def456",
			expected: "v1.73.0",
		},
		{
			name:     "image without tag",
			image:    "registry.deckhouse.io/deckhouse/ce",
			expected: "",
		},
		{
			name:     "image with port and tag",
			image:    "registry.deckhouse.io:5000/deckhouse/ce:v1.73.0",
			expected: "v1.73.0",
		},
		{
			name:     "image with multiple colons",
			image:    "registry.deckhouse.io:5000/deckhouse/ce:v1.73.0:extra",
			expected: "extra",
		},
		{
			name:     "empty string",
			image:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageTag(tt.image)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckDeckhouseVersionCompatibility(t *testing.T) {
	// Save original AppVersion
	originalVersion := app.AppVersion
	defer func() {
		app.AppVersion = originalVersion
	}()

	log.InitLogger("json")
	ctx := context.Background()

	tests := []struct {
		name           string
		dhctlVersion   string
		deckhouseVer   string
		opts           VersionCheckOptions
		expectError    bool
		expectWarnOnly bool
		setupCluster   func(*client.KubernetesClient)
	}{
		{
			name:         "matching versions",
			dhctlVersion: "1.73.0",
			deckhouseVer: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.73.0")
			},
		},
		{
			name:         "matching versions with v prefix",
			dhctlVersion: "v1.73.0",
			deckhouseVer: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "v1.73.0")
			},
		},
		{
			name:         "version mismatch",
			dhctlVersion: "1.73.0",
			deckhouseVer: "1.74.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: true,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.74.0")
			},
		},
		{
			name:         "version mismatch with AllowAnyError",
			dhctlVersion: "1.73.0",
			deckhouseVer: "1.74.0",
			opts: VersionCheckOptions{
				AllowAnyError:       true,
				AllowMissingVersion: false,
			},
			expectError:    false,
			expectWarnOnly: true,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.74.0")
			},
		},
		{
			name:         "local build skips check",
			dhctlVersion: "local",
			deckhouseVer: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.73.0")
			},
		},
		{
			name:         "dev build skips check",
			dhctlVersion: "dev",
			deckhouseVer: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.73.0")
			},
		},
		{
			name:         "missing version with AllowMissingVersion",
			dhctlVersion: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: true,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up any deployment
			},
		},
		{
			name:         "missing version without AllowMissingVersion",
			dhctlVersion: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: true,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up any deployment
			},
		},
		{
			name:         "error getting version with AllowAnyError",
			dhctlVersion: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       true,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up any deployment - will cause error
			},
		},
		{
			name:         "error getting version with AllowMissingVersion",
			dhctlVersion: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: true,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up any deployment - will cause error
			},
		},
		{
			name:         "version from DeckhouseRelease",
			dhctlVersion: "1.73.0",
			deckhouseVer: "1.73.0",
			opts: VersionCheckOptions{
				AllowAnyError:       false,
				AllowMissingVersion: false,
			},
			expectError: false,
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.AppVersion = tt.dhctlVersion
			// Configure fake client with GVR mapping for DeckhouseRelease to avoid panic when listing
			gvrMap := map[schema.GroupVersionResource]string{
				v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
			}
			fakeClient := client.NewFakeKubernetesClientWithListGVR(gvrMap)
			if tt.setupCluster != nil {
				tt.setupCluster(fakeClient)
			}

			err := CheckDeckhouseVersionCompatibility(ctx, fakeClient, tt.opts)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectWarnOnly {
					require.Contains(t, err.Error(), "Version mismatch")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetDeckhouseVersionFromCluster(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupCluster func(*client.KubernetesClient)
		expectedVer  string
		expectError  bool
	}{
		{
			name: "version from DeckhouseRelease",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
		{
			name: "fallback to deployment when release not found",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.73.0")
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
		{
			name: "prefer release over deployment",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
				setupDeploymentWithVersion(kubeCl, "1.74.0")
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
		{
			name: "error when both fail",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up anything
			},
			expectError: true,
		},
		{
			name: "skip non-deployed releases",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.72.0", v1alpha1.PhasePending)
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Configure fake client with GVR mapping for DeckhouseRelease to avoid panic when listing
			gvrMap := map[schema.GroupVersionResource]string{
				v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
			}
			fakeClient := client.NewFakeKubernetesClientWithListGVR(gvrMap)
			if tt.setupCluster != nil {
				tt.setupCluster(fakeClient)
			}

			version, err := getDeckhouseVersionFromCluster(ctx, fakeClient)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, version)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedVer, version)
			}
		})
	}
}

func TestGetDeckhouseVersionFromRelease(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupCluster func(*client.KubernetesClient)
		expectedVer  string
		expectError  bool
	}{
		{
			name: "find deployed release",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
		{
			name: "skip non-deployed releases",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.72.0", v1alpha1.PhasePending)
				setupDeckhouseRelease(kubeCl, "1.71.0", v1alpha1.PhaseSuperseded)
			},
			expectError: true,
		},
		{
			name: "no releases found",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				// Don't set up any releases
			},
			expectError: true,
		},
		{
			name: "multiple deployed releases - return first",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				gvrMap := map[schema.GroupVersionResource]string{
					v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
				}
				fakeClientWithGVR := client.NewFakeKubernetesClientWithListGVR(gvrMap)
				kubeCl.KubeClient = fakeClientWithGVR.KubeClient
				setupDeckhouseRelease(kubeCl, "1.73.0", v1alpha1.PhaseDeployed)
				setupDeckhouseRelease(kubeCl, "1.74.0", v1alpha1.PhaseDeployed)
			},
			expectedVer: "1.73.0", // Should return first found
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Configure fake client with GVR mapping for DeckhouseRelease to avoid panic when listing
			gvrMap := map[schema.GroupVersionResource]string{
				v1alpha1.DeckhouseReleaseGVR: "DeckhouseReleaseList",
			}
			fakeClient := client.NewFakeKubernetesClientWithListGVR(gvrMap)
			if tt.setupCluster != nil {
				tt.setupCluster(fakeClient)
			}

			version, err := getDeckhouseVersionFromRelease(ctx, fakeClient)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, version)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedVer, version)
			}
		})
	}
}

func TestGetDeckhouseVersionFromDeployment(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupCluster func(*client.KubernetesClient)
		expectedVer  string
		expectError  bool
	}{
		{
			name: "extract version from deployment",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "1.73.0")
			},
			expectedVer: "1.73.0",
			expectError: false,
		},
		{
			name: "extract version with v prefix",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				setupDeploymentWithVersion(kubeCl, "v1.73.0")
			},
			expectedVer: "v1.73.0",
			expectError: false,
		},
		{
			name: "extract version from image with digest",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deckhouse",
						Namespace: "d8-system",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Image: "registry.deckhouse.io/deckhouse/ce:v1.73.0@sha256:abc123",
									},
								},
							},
						},
					},
				}
				kubeCl.AppsV1().Deployments("d8-system").Create(ctx, deployment, metav1.CreateOptions{})
			},
			expectedVer: "v1.73.0",
			expectError: false,
		},
		{
			name: "deployment not found",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				// Don't set up deployment
			},
			expectError: true,
		},
		{
			name: "deployment without containers",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deckhouse",
						Namespace: "d8-system",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{},
							},
						},
					},
				}
				kubeCl.AppsV1().Deployments("d8-system").Create(ctx, deployment, metav1.CreateOptions{})
			},
			expectError: true,
		},
		{
			name: "image without tag",
			setupCluster: func(kubeCl *client.KubernetesClient) {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deckhouse",
						Namespace: "d8-system",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Image: "registry.deckhouse.io/deckhouse/ce@sha256:abc123",
									},
								},
							},
						},
					},
				}
				kubeCl.AppsV1().Deployments("d8-system").Create(ctx, deployment, metav1.CreateOptions{})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClient()
			if tt.setupCluster != nil {
				tt.setupCluster(fakeClient)
			}

			version, err := getDeckhouseVersionFromDeployment(ctx, fakeClient)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, version)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedVer, version)
			}
		})
	}
}

// Helper functions

func setupDeploymentWithVersion(kubeCl *client.KubernetesClient, version string) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse",
			Namespace: "d8-system",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: fmt.Sprintf("registry.deckhouse.io/deckhouse/ce:%s", version),
						},
					},
				},
			},
		},
	}
	kubeCl.AppsV1().Deployments("d8-system").Create(ctx, deployment, metav1.CreateOptions{})
}

func setupDeckhouseRelease(kubeCl *client.KubernetesClient, version string, phase string) {
	ctx := context.Background()
	gvr := v1alpha1.DeckhouseReleaseGVR

	release := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "DeckhouseRelease",
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("v%s", version),
			},
			"spec": map[string]interface{}{
				"version": version,
			},
			"status": map[string]interface{}{
				"phase": phase,
			},
		},
	}

	_, err := kubeCl.Dynamic().Resource(gvr).Create(ctx, release, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		// Ignore errors for test setup
	}
}

func init() {
	// Set up test environment
	if os.Getenv("DHCTL_TEST") == "" {
		os.Setenv("DHCTL_TEST", "yes")
	}
}
