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

package releaseupdater

import (
	"context"
	"testing"

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestDeckhouseReleaseRequirementsChecker(t *testing.T) {
	logger := log.NewNop()
	enabledModules := []string{"prometheus", "cert-manager"}

	// Setup common scheme for all tests
	setupScheme := func() *runtime.Scheme {
		sc := runtime.NewScheme()
		_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
		_ = corev1.AddToScheme(sc)
		return sc
	}

	t.Run("Release without requirements - installation passes", func(t *testing.T) {
		// Setup
		sc := setupScheme()

		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release without requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert
		assert.Empty(t, reasons, "Release without requirements should pass without errors")
	})

	t.Run("Release with migratedModules - all modules exist in registry - installation passes", func(t *testing.T) {
		// Setup
		sc := setupScheme()

		// Create ModuleSource with available modules
		moduleSource := &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "deckhouse",
			},
			Status: v1alpha1.ModuleSourceStatus{
				AvailableModules: []v1alpha1.AvailableModule{
					{
						Name:      "test-module-1",
						PullError: "", // No pull error means module is available
					},
					{
						Name:      "test-module-2",
						PullError: "", // No pull error means module is available
					},
				},
			},
		}

		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret, moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release with migratedModules requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
				Requirements: map[string]string{
					MigratedModulesRequirementFieldName: "test-module-1, test-module-2",
				},
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert
		assert.Empty(t, reasons, "Release with all migrated modules available should pass without errors")
	})

	t.Run("Release with migratedModules - one module missing in registry - installation fails", func(t *testing.T) {
		// Setup
		sc := setupScheme()

		// Create ModuleSource with only one available module
		moduleSource := &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "deckhouse",
			},
			Status: v1alpha1.ModuleSourceStatus{
				AvailableModules: []v1alpha1.AvailableModule{
					{
						Name:      "test-module-1",
						PullError: "", // No pull error means module is available
					},
					// test-module-2 is missing from available modules
				},
			},
		}

		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret, moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release with migratedModules requirements (including missing module)
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
				Requirements: map[string]string{
					MigratedModulesRequirementFieldName: "test-module-1, test-module-2",
				},
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert
		assert.Len(t, reasons, 1, "Should have one requirement failure")
		assert.Equal(t, "migrated modules check", reasons[0].Reason)
		assert.Contains(t, reasons[0].Message, "test-module-2")
		assert.Contains(t, reasons[0].Message, "not found in any ModuleSource registry")

		// Note: Metrics testing would require more complex setup to verify
		// that the MigratedModuleNotFoundMetricName metric is properly set
	})

	t.Run("Release with migratedModules - module has pull error - installation fails", func(t *testing.T) {
		// Setup
		sc := setupScheme()

		// Create ModuleSource with module that has pull error
		moduleSource := &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "deckhouse",
			},
			Status: v1alpha1.ModuleSourceStatus{
				AvailableModules: []v1alpha1.AvailableModule{
					{
						Name:      "test-module-1",
						PullError: "failed to pull module", // Module has pull error
					},
				},
			},
		}

		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret, moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release with migratedModules requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
				Requirements: map[string]string{
					MigratedModulesRequirementFieldName: "test-module-1",
				},
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert
		assert.Len(t, reasons, 1, "Should have one requirement failure")
		assert.Equal(t, "migrated modules check", reasons[0].Reason)
		assert.Contains(t, reasons[0].Message, "test-module-1")
		assert.Contains(t, reasons[0].Message, "not found in any ModuleSource registry")
	})

	t.Run("Release with empty migratedModules string - installation passes", func(t *testing.T) {
		// Setup
		sc := setupScheme()
		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release with empty migratedModules requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
				Requirements: map[string]string{
					MigratedModulesRequirementFieldName: "",
				},
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert
		assert.Empty(t, reasons, "Release with empty migratedModules string should pass without errors")
	})

	t.Run("Release with whitespace-only migratedModules - installation fails due to empty module names", func(t *testing.T) {
		// Setup
		sc := setupScheme()
		clusterConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(clusterConfigSecret).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		exts := extenders.NewExtendersStack(nil, "", logger)

		// Create release with whitespace-only migratedModules requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
				Requirements: map[string]string{
					MigratedModulesRequirementFieldName: "   ,  ,   ",
				},
			},
		}

		// Create checker
		checker, err := NewDeckhouseReleaseRequirementsChecker(cl, enabledModules, exts, metricStorage, logger)
		require.NoError(t, err)

		// Test
		reasons := checker.MetRequirements(context.TODO(), release)

		// Assert - the current implementation doesn't filter empty strings after trimming,
		// so this will fail as expected since empty module names can't be found
		assert.Len(t, reasons, 1, "Should have one requirement failure for empty module name")
		assert.Equal(t, "migrated modules check", reasons[0].Reason)
		assert.Contains(t, reasons[0].Message, "not found in any ModuleSource registry")
	})
}
