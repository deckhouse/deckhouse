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

package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestModuleIsInstalled(t *testing.T) {
	assert.False(t, (&Module{}).IsInstalled())
	assert.True(t, (&Module{Spec: ModuleSpec{PackageVersion: "v1.0.0"}}).IsInstalled())
}

func TestModuleHasCatalogPhase(t *testing.T) {
	for _, phase := range []string{
		v1alpha1.ModulePhaseAvailable,
		v1alpha1.ModulePhaseConflict,
		v1alpha1.ModulePhaseDownloading,
		v1alpha1.ModulePhaseDownloadingError,
	} {
		assert.True(t, (&Module{Status: ModuleStatus{Phase: phase}}).HasCatalogPhase(), phase)
	}

	for _, phase := range []string{"", v1alpha1.ModulePhaseReady, v1alpha1.ModulePhaseError, v1alpha1.ModulePhaseInstalling} {
		assert.False(t, (&Module{Status: ModuleStatus{Phase: phase}}).HasCatalogPhase(), phase)
	}
}

func TestModuleSetNotInstalledStatus(t *testing.T) {
	module := &Module{}
	module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonEnabled)

	module.SetNotInstalledStatus()

	assert.Equal(t, v1alpha1.ModulePhaseAvailable, module.Status.Phase)
	assertCondition(t, module, v1alpha1.ModuleConditionEnabledByModuleManager, metav1.ConditionFalse, v1alpha1.ModuleReasonDisabled, "")
	assertCondition(t, module, v1alpha1.ModuleConditionIsReady, metav1.ConditionFalse, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
	// the config condition belongs to the config controller and stays
	assertCondition(t, module, v1alpha1.ModuleConditionEnabledByModuleConfig, metav1.ConditionTrue, v1alpha1.ModuleReasonEnabled, "")
}

func TestModuleSetConflictStatus(t *testing.T) {
	module := &Module{}

	module.SetConflictStatus()

	assert.Equal(t, v1alpha1.ModulePhaseConflict, module.Status.Phase)
	assertCondition(t, module, v1alpha1.ModuleConditionEnabledByModuleManager, metav1.ConditionFalse, v1alpha1.ModuleReasonDisabled, "")
	assertCondition(t, module, v1alpha1.ModuleConditionIsReady, metav1.ConditionFalse, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)
}

func assertCondition(t *testing.T, module *Module, condType string, status metav1.ConditionStatus, reason, message string) {
	t.Helper()

	cond := meta.FindStatusCondition(module.Status.Conditions, condType)
	require.NotNil(t, cond, condType)
	assert.Equal(t, status, cond.Status, condType)
	assert.Equal(t, reason, cond.Reason, condType)
	assert.Equal(t, message, cond.Message, condType)
}
