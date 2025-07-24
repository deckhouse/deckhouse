/*
Copyright 2024 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestMigratedModulesCheck(t *testing.T) {
	logger := log.NewNop()

	// Setup common scheme for all tests
	setupScheme := func() *runtime.Scheme {
		sc := runtime.NewScheme()
		_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
		return sc
	}

	t.Run("Release without migratedModules - installation passes", func(t *testing.T) {
		// Setup
		sc := setupScheme()
		cl := fake.NewClientBuilder().WithScheme(sc).Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

		// Create release without migratedModules requirements
		release := &v1alpha1.DeckhouseRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1.50.0",
			},
			Spec: v1alpha1.DeckhouseReleaseSpec{
				Version: "v1.50.0",
			},
		}

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.NoError(t, err, "Release without migratedModules should pass without errors")
	})

	t.Run("Release with empty migratedModules string - installation passes", func(t *testing.T) {
		// Setup
		sc := setupScheme()
		cl := fake.NewClientBuilder().WithScheme(sc).Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.NoError(t, err, "Release with empty migratedModules string should pass without errors")
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

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.NoError(t, err, "Release with all migrated modules available should pass without errors")
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

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.Error(t, err, "Should have an error for missing module")
		assert.Contains(t, err.Error(), "test-module-2")
		assert.Contains(t, err.Error(), "not found in any ModuleSource registry")
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

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(moduleSource).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.Error(t, err, "Should have an error for module with pull error")
		assert.Contains(t, err.Error(), "test-module-1")
		assert.Contains(t, err.Error(), "not found in any ModuleSource registry")
	})

	t.Run("Release with whitespace-only migratedModules - installation fails due to empty module names", func(t *testing.T) {
		// Setup
		sc := setupScheme()
		cl := fake.NewClientBuilder().WithScheme(sc).Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert - the current implementation doesn't filter empty strings after trimming,
		// so this will fail as expected since empty module names can't be found
		assert.Error(t, err, "Should have an error for empty module name")
		assert.Contains(t, err.Error(), "not found in any ModuleSource registry")
	})

	t.Run("Release with multiple ModuleSources - module found in second source", func(t *testing.T) {
		// Setup
		sc := setupScheme()

		// Create first ModuleSource without the required module
		moduleSource1 := &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "source-1",
			},
			Status: v1alpha1.ModuleSourceStatus{
				AvailableModules: []v1alpha1.AvailableModule{
					{
						Name:      "other-module",
						PullError: "",
					},
				},
			},
		}

		// Create second ModuleSource with the required module
		moduleSource2 := &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "source-2",
			},
			Status: v1alpha1.ModuleSourceStatus{
				AvailableModules: []v1alpha1.AvailableModule{
					{
						Name:      "test-module-1",
						PullError: "", // No pull error means module is available
					},
				},
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(sc).
			WithObjects(moduleSource1, moduleSource2).
			Build()

		metricStorage := metricstorage.NewMetricStorage(context.Background(), "", false, logger)
		checker := newMigratedModulesCheck(cl, metricStorage, logger)

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

		// Test
		err := checker.Verify(context.TODO(), release)

		// Assert
		assert.NoError(t, err, "Module should be found in second ModuleSource")
	})
}