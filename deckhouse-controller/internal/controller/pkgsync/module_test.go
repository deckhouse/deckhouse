// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkgsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

func testOverride(module, tag string) *v1alpha2.ModulePullOverride {
	return &v1alpha2.ModulePullOverride{
		ObjectMeta: metav1.ObjectMeta{Name: module},
		Spec:       v1alpha2.ModulePullOverrideSpec{ImageTag: tag},
	}
}

func testConfig(module string, spec v1alpha1.ModuleConfigSpec) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: module},
		Spec:       spec,
	}
}

func testModule(name, repository, version string, annotations map[string]string) *v1alpha2.Module {
	return &v1alpha2.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
		Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: repository, PackageVersion: version},
	}
}

func testSourceOffering(name string, modules ...string) *v1alpha1.ModuleSource {
	source := testModuleSource(name, "registry.example.io/"+name)
	for _, module := range modules {
		source.Status.AvailableModules = append(source.Status.AvailableModules, v1alpha1.AvailableModule{Name: module})
	}

	return source
}

func TestSyncModulesPlacement(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
	writeModuleYAML(t, filepath.Join(dir, "910-console"), "name: console\n")
	// a directory without a definition is named after itself
	require.NoError(t, writeDir(filepath.Join(dir, "920-bare")))

	s, cl := newTestSyncer(t, "v1.80.3+build", dir,
		// the image wins over the override of the same module
		testOverride("console", "pr123"),
		// a dev module: the config names its source
		testOverride("parca", "main"),
		testConfig("parca", v1alpha1.ModuleConfigSpec{Source: "deckhouse"}),
		// two deployed releases: the newest places the module, the other is superseded
		testRelease("upmeter", "deckhouse", "1.2.0", v1alpha1.ModuleReleasePhaseDeployed),
		testRelease("upmeter", "deckhouse", "1.3.0", v1alpha1.ModuleReleasePhaseDeployed),
		testRelease("upmeter", "deckhouse", "1.4.0", v1alpha1.ModuleReleasePhasePending),
		// a module offered by a single source and pinned to a tag
		testOverride("solo", "dev"),
		testSourceOffering("external", "solo"),
		// an override whose repository no resource names is skipped
		testOverride("orphan", "dev"),
	)

	require.NoError(t, s.syncModules(ctx))

	assert.ElementsMatch(t, []string{"bare", "console", "echo", "parca", "solo", "upmeter"}, listModuleNames(t, cl))

	echo := getModule(t, cl, "echo")
	assert.Equal(t, "embedded", echo.Spec.PackageRepositoryName)
	assert.Equal(t, "v1.80.3", echo.Spec.PackageVersion)
	assert.True(t, echo.IsEmbedded())
	assert.False(t, echo.IsDev())

	console := getModule(t, cl, "console")
	assert.True(t, console.IsEmbedded())
	assert.False(t, console.IsDev())
	assert.Equal(t, "v1.80.3", console.Spec.PackageVersion)

	parca := getModule(t, cl, "parca")
	assert.True(t, parca.IsDev())
	assert.Equal(t, "main", parca.Spec.PackageVersion)
	assert.Equal(t, "deckhouse-modules", parca.Spec.PackageRepositoryName)

	solo := getModule(t, cl, "solo")
	assert.True(t, solo.IsDev())
	assert.Equal(t, "external", solo.Spec.PackageRepositoryName)

	upmeter := getModule(t, cl, "upmeter")
	assert.False(t, upmeter.IsDev())
	assert.Equal(t, "v1.3.0", upmeter.Spec.PackageVersion)
	assert.Equal(t, "deckhouse-modules", upmeter.Spec.PackageRepositoryName)

	superseded := new(v1alpha1.ModuleRelease)
	require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "upmeter-v1.2.0"}, superseded))
	assert.Equal(t, v1alpha1.ModuleReleasePhaseSuperseded, superseded.Status.Phase)

	newest := new(v1alpha1.ModuleRelease)
	require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "upmeter-v1.3.0"}, newest))
	assert.Equal(t, v1alpha1.ModuleReleasePhaseDeployed, newest.Status.Phase)
}

func TestSyncModulesBrokenEmbeddedDefinitionFails(t *testing.T) {
	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: [broken\n")

	s, _ := newTestSyncer(t, "v1.80.0", dir)

	err := s.syncModules(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "900-echo")
}

func TestSyncModulesConfigOverlay(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

	settings := v1alpha1.MakeMappedFields(map[string]any{"replicas": 2})
	config := testConfig("echo", v1alpha1.ModuleConfigSpec{
		Enabled:      ptr.To(true),
		Version:      2,
		Settings:     settings,
		Maintenance:  "NoResourceReconciliation",
		UpdatePolicy: "test-alpha",
		Source:       "external",
	})

	s, cl := newTestSyncer(t, "v1.80.0", dir, config)

	require.NoError(t, s.syncModules(ctx))

	echo := getModule(t, cl, "echo")
	assert.Equal(t, ptr.To(true), echo.Spec.Enabled)
	assert.Equal(t, 2, echo.Spec.SettingsVersion)
	assert.Equal(t, settings.GetMap(), echo.Spec.Settings.GetMap())
	assert.Equal(t, "NoResourceReconciliation", echo.Spec.Maintenance)
	assert.Equal(t, "test-alpha", echo.Spec.UpdatePolicy)
	// the config's source never overrides where the package comes from
	assert.Equal(t, "embedded", echo.Spec.PackageRepositoryName)

	// the config fields die with the config
	require.NoError(t, cl.Delete(ctx, config))
	require.NoError(t, s.syncModules(ctx))

	echo = getModule(t, cl, "echo")
	assert.Nil(t, echo.Spec.Enabled)
	assert.Zero(t, echo.Spec.SettingsVersion)
	assert.True(t, echo.Spec.Settings.IsEmpty())
	assert.Empty(t, echo.Spec.Maintenance)
	assert.Empty(t, echo.Spec.UpdatePolicy)
	assert.Equal(t, "v1.80.0", echo.Spec.PackageVersion)
}

func TestSyncModulesDisposesUnbackedModules(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

	s, cl := newTestSyncer(t, "v1.80.0", dir,
		// an object of the old catalog: no version, nothing backs it
		testModule("offered", "", "", nil),
		// an embedded module the image stopped shipping
		testModule("dropped", "embedded", "v1.79.0", map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"}),
		// an embedded module a real repository took over keeps the spec another writer gave it
		testModule("migrated", "deckhouse-modules", "v2.0.0", map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"}),
		// a downloaded module with a version and no origin keeps running from its files: only the dev mark goes
		testModule("stale", "deckhouse-modules", "v0.1.0", map[string]string{v1alpha2.ModuleAnnotationDev: "true"}),
		// a downloaded module whose files are gone was uninstalled with its release
		testModule("gone", "deckhouse-modules", "v0.2.0", nil),
	)
	require.NoError(t, os.MkdirAll(filepath.Join(s.downloadedModulesDir, "stale"), 0o755))

	require.NoError(t, s.syncModules(ctx))

	assert.ElementsMatch(t, []string{"echo", "migrated", "stale"}, listModuleNames(t, cl))

	migrated := getModule(t, cl, "migrated")
	assert.False(t, migrated.IsEmbedded())
	assert.Equal(t, "v2.0.0", migrated.Spec.PackageVersion)

	stale := getModule(t, cl, "stale")
	assert.False(t, stale.IsDev())
	assert.Equal(t, "v0.1.0", stale.Spec.PackageVersion)

	for _, name := range []string{"offered", "dropped", "gone"} {
		err := cl.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha2.Module))
		assert.True(t, apierrors.IsNotFound(err), name)
	}
}

func TestSyncModulesNormalizesConditions(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

	s, cl := newTestSyncer(t, "v1.80.0", dir)

	// the old stack's conditions come without a reason
	module := testModule("echo", "embedded", "v1.80.0", map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"})
	require.NoError(t, cl.Create(ctx, module))
	module.Status.Conditions = []metav1.Condition{
		{Type: v1alpha1.ModuleConditionEnabledByModuleConfig, Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now()},
		{Type: v1alpha1.ModuleConditionEnabledByModuleManager, Status: metav1.ConditionFalse, LastTransitionTime: metav1.Now()},
		{Type: v1alpha1.ModuleConditionIsReady, Status: metav1.ConditionFalse, Reason: "Disabled", Message: "disabled", LastTransitionTime: metav1.Now()},
		{Type: v1alpha1.ModuleConditionLastReleaseDeployed, Status: metav1.ConditionUnknown, LastTransitionTime: metav1.Now()},
	}
	require.NoError(t, cl.Status().Update(ctx, module))

	require.NoError(t, s.syncModules(ctx))

	reasons := make(map[string]string)
	for _, cond := range getModule(t, cl, "echo").Status.Conditions {
		reasons[cond.Type] = cond.Reason
	}

	assert.Equal(t, map[string]string{
		v1alpha1.ModuleConditionEnabledByModuleConfig:  "Enabled",
		v1alpha1.ModuleConditionEnabledByModuleManager: "Disabled",
		v1alpha1.ModuleConditionIsReady:                "Disabled",
		v1alpha1.ModuleConditionLastReleaseDeployed:    "Unknown",
	}, reasons)

	// a second pass finds nothing to write
	rv := getModule(t, cl, "echo").ResourceVersion
	require.NoError(t, s.syncModules(ctx))
	assert.Equal(t, rv, getModule(t, cl, "echo").ResourceVersion)
}

func TestModuleRepository(t *testing.T) {
	ctx := context.Background()

	_, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
		testModule("placed", "deckhouse-modules", "v1.0.0", nil),
		testModule("embedded", "embedded", "v1.80.0", map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"}),
		testConfig("embedded", v1alpha1.ModuleConfigSpec{Source: "external"}),
		testConfig("configured", v1alpha1.ModuleConfigSpec{Source: "deckhouse"}),
		testRelease("released", "external", "1.0.0", v1alpha1.ModuleReleasePhaseDeployed),
		testRelease("pending", "deckhouse", "1.0.0", v1alpha1.ModuleReleasePhasePending),
		testSourceOffering("only", "single", "ambiguous"),
		testSourceOffering("other", "ambiguous"),
	)

	cases := []struct {
		module string
		want   string
	}{
		{module: "placed", want: "deckhouse-modules"},
		{module: "embedded", want: "external"},
		{module: "configured", want: "deckhouse-modules"},
		{module: "released", want: "external"},
		{module: "pending", want: "deckhouse-modules"},
		{module: "single", want: "only"},
		{module: "ambiguous", want: ""},
		{module: "unknown", want: ""},
	}

	for _, c := range cases {
		got, err := ModuleRepository(ctx, cl, c.module)
		require.NoError(t, err, c.module)
		assert.Equal(t, c.want, got, c.module)
	}
}

// writeDir creates an empty module directory.
func writeDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
