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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func enabledModule(name string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: v1alpha1.ModuleStatus{
			Conditions: []v1alpha1.ModuleCondition{
				{Type: v1alpha1.ModuleConditionEnabledByModuleManager, Status: corev1.ConditionTrue},
			},
		},
	}
}

func moduleReleaseWithPhase(moduleName, phase string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:   moduleName + "-v1.0.0",
			Labels: map[string]string{v1alpha1.ModuleReleaseLabelModule: moduleName},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: moduleName, Version: "1.0.0"},
		Status: v1alpha1.ModuleReleaseStatus{Phase: phase},
	}
}

func moduleSourceWith(name string, modules []v1alpha1.AvailableModule) *v1alpha1.ModuleSource {
	return &v1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     v1alpha1.ModuleSourceStatus{AvailableModules: modules},
	}
}

// TestMigratedModulesCheck_DistinctErrors locks down that the migrated-modules
// gate distinguishes the terminal "no source offers the module at all" case from
// the transient "available but not downloaded yet" case, so the operator gets the
// right diagnostic instead of a misleading "wait for it to download" for a module
// that no source will ever provide.
func TestMigratedModulesCheck_DistinctErrors(t *testing.T) {
	const moduleName = "test-module-1"

	tests := []struct {
		name        string
		objects     []client.Object
		wantErr     bool
		wantErrText string
	}{
		{
			name: "not available in any source -> not found in any ModuleSource registry",
			objects: []client.Object{
				enabledModule(moduleName),
				moduleSourceWith("src", []v1alpha1.AvailableModule{{Name: "other-module"}}),
			},
			wantErr:     true,
			wantErrText: "not found in any ModuleSource registry",
		},
		{
			name: "available with a pull error -> treated as not found",
			objects: []client.Object{
				enabledModule(moduleName),
				moduleSourceWith("src", []v1alpha1.AvailableModule{{Name: moduleName, Error: "pull failed"}}),
			},
			wantErr:     true,
			wantErrText: "not found in any ModuleSource registry",
		},
		{
			name: "available in a source but not downloaded yet -> not pre-downloaded yet",
			objects: []client.Object{
				enabledModule(moduleName),
				moduleSourceWith("src", []v1alpha1.AvailableModule{{Name: moduleName}}),
				moduleReleaseWithPhase(moduleName, v1alpha1.ModuleReleasePhasePending),
			},
			wantErr:     true,
			wantErrText: "is not pre-downloaded yet",
		},
		{
			name: "deployed release -> allowed",
			objects: []client.Object{
				enabledModule(moduleName),
				moduleSourceWith("src", []v1alpha1.AvailableModule{{Name: moduleName}}),
				moduleReleaseWithPhase(moduleName, v1alpha1.ModuleReleasePhaseDeployed),
			},
			wantErr: false,
		},
		{
			name: "disabled module is skipped -> allowed even with no source",
			objects: []client.Object{
				&v1alpha1.Module{
					ObjectMeta: metav1.ObjectMeta{Name: moduleName},
					Status: v1alpha1.ModuleStatus{Conditions: []v1alpha1.ModuleCondition{
						{Type: v1alpha1.ModuleConditionEnabledByModuleManager, Status: corev1.ConditionFalse},
					}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

			check := newMigratedModulesCheck(fakeClient, metricStorage, log.NewNop())

			dr := &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version:      "v1.60.0",
					Requirements: map[string]string{MigratedModulesRequirementFieldName: moduleName},
				},
			}

			err := check.Verify(context.Background(), dr)

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrText)
		})
	}
}
