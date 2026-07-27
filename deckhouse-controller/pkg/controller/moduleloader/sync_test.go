// Copyright 2024 Flant JSC
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

package moduleloader

import (
	"context"
	"testing"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	installermock "github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer/mock"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	testNodeName = "dev-master-0"
	testRepo     = "dev-registry.example.io/deckhouse/modules"
)

// installerCall records a single (module, version) installer invocation so a test
// can assert which path (Restore vs StageFromRegistry) a module took.
type installerCall struct {
	module  string
	version string
}

// installerCalls captures everything the loader asked the installer to do.
type installerCalls struct {
	restore           []installerCall
	stageFromRegistry []installerCall
	uninstall         []string
}

// newRecordingInstaller returns a mock installer that records its calls and reports
// the modules in embedded as having an embedded copy still shipped on the filesystem.
func newRecordingInstaller(calls *installerCalls, embedded map[string]bool) *installermock.Installer {
	return &installermock.Installer{
		IsEmbeddedPresentFunc: func(module string) bool { return embedded[module] },
		RestoreFunc: func(_ context.Context, _ *v1alpha1.ModuleSource, module, version string) error {
			calls.restore = append(calls.restore, installerCall{module, version})
			return nil
		},
		StageFromRegistryFunc: func(_ context.Context, _ *v1alpha1.ModuleSource, module, version string) error {
			calls.stageFromRegistry = append(calls.stageFromRegistry, installerCall{module, version})
			return nil
		},
		UninstallFunc: func(_ context.Context, module string) error {
			calls.uninstall = append(calls.uninstall, module)
			return nil
		},
	}
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	sc := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(sc))
	require.NoError(t, v1alpha2.SchemeBuilder.AddToScheme(sc))
	require.NoError(t, corev1.AddToScheme(sc))
	return sc
}

func newTestLoader(t *testing.T, inst Installer, objects ...client.Object) *Loader {
	t.Helper()

	cl := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.Module{}, &v1alpha1.ModuleRelease{}, &v1alpha1.ModuleSource{}, &v1alpha2.ModulePullOverride{}).
		Build()

	tmp := t.TempDir()

	return &Loader{
		client:               cl,
		logger:               log.NewNop(),
		installer:            inst,
		registries:           make(map[string]*addonmodules.Registry),
		dependencyContainer:  dependency.NewDependencyContainer(),
		downloadedModulesDir: tmp,
		symlinksDir:          tmp + "/modules",
	}
}

// --- object builders -------------------------------------------------------

func testModuleSource(name, repo string) *v1alpha1.ModuleSource {
	return &v1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ModuleSourceSpec{
			Registry: v1alpha1.ModuleSourceSpecRegistry{
				Repo:      repo,
				DockerCFG: "YXNiCg==",
				Scheme:    "HTTP",
			},
		},
	}
}

func testModule(name, source string, availableSources ...string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Properties: v1alpha1.ModuleProperties{
			Source:           source,
			AvailableSources: availableSources,
			Weight:           900,
		},
	}
}

func testDeployedRelease(module, sourceName, version string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: module + "-v" + version,
			Labels: map[string]string{
				"module":                          module,
				"source":                          sourceName,
				v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
			},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: module, Version: version, Weight: 900},
		Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhaseDeployed},
	}
}

// enabled marks the module as enabled by ModuleConfig, which restoreModulesByOverrides
// requires before it will restore an overridden module.
func enabled(module *v1alpha1.Module) *v1alpha1.Module {
	module.Status.Conditions = append(module.Status.Conditions, v1alpha1.ModuleCondition{
		Type:   v1alpha1.ModuleConditionEnabledByModuleConfig,
		Status: corev1.ConditionTrue,
	})
	return module
}

func testReadyMPO(name, imageTag, deployedOn string) *v1alpha2.ModulePullOverride {
	return &v1alpha2.ModulePullOverride{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{v1alpha1.ModulePullOverrideAnnotationDeployedOn: deployedOn},
		},
		Spec:   v1alpha2.ModulePullOverrideSpec{ImageTag: imageTag},
		Status: v1alpha2.ModulePullOverrideStatus{Message: v1alpha1.ModulePullOverrideMessageReady},
	}
}

func getModule(t *testing.T, l *Loader, name string) *v1alpha1.Module {
	t.Helper()
	module := new(v1alpha1.Module)
	require.NoError(t, l.client.Get(context.Background(), client.ObjectKey{Name: name}, module))
	return module
}

func getRelease(t *testing.T, l *Loader, name string) *v1alpha1.ModuleRelease {
	t.Helper()
	release := new(v1alpha1.ModuleRelease)
	require.NoError(t, l.client.Get(context.Background(), client.ObjectKey{Name: name}, release))
	return release
}

// --- restoreModulesByReleases ----------------------------------------------

func TestRestoreModulesByReleases(t *testing.T) {
	t.Run("non-embedded module is activated and pinned to its source registry", func(t *testing.T) {
		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", "losev-test", "losev-test"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		require.NoError(t, l.restoreModulesByReleases(context.Background()))

		assert.Equal(t, []installerCall{{"echo", "v1.0.0"}}, calls.restore, "non-embedded module must be restored")
		assert.Empty(t, calls.stageFromRegistry, "non-embedded module must not be staged")

		reg, ok := l.registries["echo"]
		require.True(t, ok, "registry override must be set for an activated module")
		assert.Equal(t, testRepo, reg.Base)

		assert.Equal(t, "v1.0.0", getModule(t, l, "echo").Properties.Version)
	})

	// Regression: a module whose embedded copy is still shipped must be staged
	// (downloaded, not activated) and must NOT receive the source registry override,
	// otherwise it renders <sourceRepo>/modules/<name>@<embeddedDigest> and fails with
	// ImagePullBackOff. See moduleloader/sync.go restoreModulesByReleases.
	t.Run("embedded module is staged and keeps the embedded registry", func(t *testing.T) {
		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, map[string]bool{"echo": true}),
			testModuleSource("example", testRepo),
			testModule("echo", "losev-test", "losev-test"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		require.NoError(t, l.restoreModulesByReleases(context.Background()))

		assert.Equal(t, []installerCall{{"echo", "v1.0.0"}}, calls.stageFromRegistry, "embedded module must be staged")
		assert.Empty(t, calls.restore, "embedded module must not be activated")

		_, ok := l.registries["echo"]
		assert.False(t, ok, "embedded module must keep its embedded registry (no source override)")

		// version is still tracked even while the module stays embedded
		assert.Equal(t, "v1.0.0", getModule(t, l, "echo").Properties.Version)
	})

	t.Run("migrated module switches active source once the embedded copy is gone", func(t *testing.T) {
		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", v1alpha1.ModuleSourceEmbedded, "example"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		require.NoError(t, l.restoreModulesByReleases(context.Background()))

		assert.Equal(t, []installerCall{{"echo", "v1.0.0"}}, calls.restore)
		reg, ok := l.registries["echo"]
		require.True(t, ok)
		assert.Equal(t, testRepo, reg.Base)

		// the embedded sentinel must be flipped to the real source
		assert.Equal(t, "example", getModule(t, l, "echo").Properties.Source)
	})

	t.Run("multiple deployed releases: newest wins, older are superseded", func(t *testing.T) {
		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", "losev-test", "losev-test"),
			testDeployedRelease("echo", "example", "1.0.0"),
			testDeployedRelease("echo", "example", "1.0.2"),
			testDeployedRelease("echo", "example", "1.0.1"),
		)

		require.NoError(t, l.restoreModulesByReleases(context.Background()))

		// the module ends up pinned to the highest version
		assert.Equal(t, "v1.0.2", getModule(t, l, "echo").Properties.Version)

		assert.Equal(t, v1alpha1.ModuleReleasePhaseSuperseded, getRelease(t, l, "echo-v1.0.0").Status.Phase)
		assert.Equal(t, v1alpha1.ModuleReleasePhaseSuperseded, getRelease(t, l, "echo-v1.0.1").Status.Phase)
		assert.Equal(t, v1alpha1.ModuleReleasePhaseDeployed, getRelease(t, l, "echo-v1.0.2").Status.Phase)

		require.Len(t, calls.restore, 3, "every deployed release is processed in version order")
		assert.Equal(t, "v1.0.2", calls.restore[len(calls.restore)-1].version, "the last processed release is the newest")
	})

	t.Run("module with a pull override is skipped in the releases path", func(t *testing.T) {
		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", "losev-test", "losev-test"),
			testDeployedRelease("echo", "example", "1.0.0"),
			testReadyMPO("echo", "v1.0.0", testNodeName),
		)

		require.NoError(t, l.restoreModulesByReleases(context.Background()))

		assert.Empty(t, calls.restore, "MPO-managed module must not be restored by the releases path")
		assert.Empty(t, calls.stageFromRegistry)
		_, ok := l.registries["echo"]
		assert.False(t, ok)
	})
}

// --- restoreModulesByOverrides ---------------------------------------------

func TestRestoreModulesByOverrides(t *testing.T) {
	t.Run("non-embedded module is restored and pinned to its source registry", func(t *testing.T) {
		t.Setenv("DECKHOUSE_NODE_NAME", testNodeName)

		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			enabled(testModule("echo", "example", "example")),
			testReadyMPO("echo", "v1.0.0", testNodeName),
		)

		require.NoError(t, l.restoreModulesByOverrides(context.Background()))

		assert.Equal(t, []installerCall{{"echo", "v1.0.0"}}, calls.restore)
		assert.Empty(t, calls.uninstall, "no uninstall when deployed-on matches the current node")

		reg, ok := l.registries["echo"]
		require.True(t, ok)
		assert.Equal(t, testRepo, reg.Base)
	})

	t.Run("embedded module is skipped", func(t *testing.T) {
		t.Setenv("DECKHOUSE_NODE_NAME", testNodeName)

		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", v1alpha1.ModuleSourceEmbedded, "example"),
			testReadyMPO("echo", "v1.0.0", testNodeName),
		)

		require.NoError(t, l.restoreModulesByOverrides(context.Background()))

		assert.Empty(t, calls.restore, "embedded module must not be restored from a pull override")
	})

	t.Run("stale deployed-on annotation triggers a reinstall", func(t *testing.T) {
		t.Setenv("DECKHOUSE_NODE_NAME", testNodeName)

		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			enabled(testModule("echo", "example", "example")),
			testReadyMPO("echo", "v1.0.0", "some-old-master"),
		)

		require.NoError(t, l.restoreModulesByOverrides(context.Background()))

		assert.Equal(t, []string{"echo"}, calls.uninstall, "stale deployed-on must trigger uninstall")
		assert.Equal(t, []installerCall{{"echo", "v1.0.0"}}, calls.restore)
	})

	t.Run("not-ready override is skipped", func(t *testing.T) {
		t.Setenv("DECKHOUSE_NODE_NAME", testNodeName)

		mpo := testReadyMPO("echo", "v1.0.0", testNodeName)
		mpo.Status.Message = "Downloading"

		calls := new(installerCalls)
		l := newTestLoader(t, newRecordingInstaller(calls, nil),
			testModuleSource("example", testRepo),
			testModule("echo", "example", "example"),
			mpo,
		)

		require.NoError(t, l.restoreModulesByOverrides(context.Background()))

		assert.Empty(t, calls.restore, "an override that is not Ready must be skipped")
	})
}

// --- deleteStaleModuleReleases ---------------------------------------------

func TestDeleteStaleModuleReleases(t *testing.T) {
	staleModule := func(name, source string) *v1alpha1.Module {
		module := testModule(name, source, source)
		module.Status.Conditions = []v1alpha1.ModuleCondition{
			{
				Type:               v1alpha1.ModuleConditionEnabledByModuleConfig,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-100 * time.Hour)),
			},
		}
		return module
	}

	t.Run("releases of a long-disabled non-embedded module are pruned", func(t *testing.T) {
		l := newTestLoader(t, newRecordingInstaller(new(installerCalls), nil),
			staleModule("echo", "example"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		require.NoError(t, l.deleteStaleModuleReleases(context.Background()))

		err := l.client.Get(context.Background(), client.ObjectKey{Name: "echo-v1.0.0"}, new(v1alpha1.ModuleRelease))
		assert.True(t, apierrors.IsNotFound(err), "stale module release must be deleted")

		module := getModule(t, l, "echo")
		assert.Equal(t, v1alpha1.ModulePhaseAvailable, module.Status.Phase)
		assert.Empty(t, module.Properties.Source, "module properties must be cleared")
		assert.Equal(t, []string{"example"}, module.Properties.AvailableSources, "available sources must be preserved")
	})

	t.Run("embedded module is never pruned", func(t *testing.T) {
		l := newTestLoader(t, newRecordingInstaller(new(installerCalls), nil),
			staleModule("echo", v1alpha1.ModuleSourceEmbedded),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		require.NoError(t, l.deleteStaleModuleReleases(context.Background()))

		err := l.client.Get(context.Background(), client.ObjectKey{Name: "echo-v1.0.0"}, new(v1alpha1.ModuleRelease))
		assert.NoError(t, err, "embedded module release must be kept")
	})
}
