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

package source

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gojuno/minimock/v3"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

var manifestStub = func() (*crv1.Manifest, error) {
	return &crv1.Manifest{
		Layers: []crv1.Descriptor{},
	}, nil
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	r *reconciler

	source        string
	compareGolden bool
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type reconcilerOption func(*reconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dc = dc
	}
}

func (suite *ControllerTestSuite) setupTestController(filename string, options ...reconcilerOption) {
	suite.Seed(filename)
	suite.buildReconciler(options...)
}

func (suite *ControllerTestSuite) setupTestControllerRaw(raw string, options ...reconcilerOption) {
	suite.SeedRaw("inline.yaml", []byte(raw))
	suite.buildReconciler(options...)
}

func (suite *ControllerTestSuite) buildReconciler(options ...reconcilerOption) {
	var sources v1alpha1.ModuleSourceList
	require.NoError(suite.T(), suite.Client().List(context.TODO(), &sources))
	if len(sources.Items) > 0 {
		suite.source = sources.Items[0].Name
	}

	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

	rec := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               suite.Client(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dependency.NewDependencyContainer(),
		logger:               log.NewNop(),
		edition: &d8edition.Edition{
			Name:   "fe",
			Bundle: "Default",
		},
		metricStorage: metricStorage,
		deckhouseSettings: helpers.NewDeckhouseSettingsContainer(&helpers.DeckhouseSettings{
			Update: struct {
				Mode                   string                            `json:"mode"`
				DisruptionApprovalMode string                            `json:"disruptionApprovalMode"`
				Windows                update.Windows                    `json:"windows"`
				NotificationConfig     releaseUpdater.NotificationConfig `json:"notification"`
				BlockOnAlerts          releaseUpdater.BlockOnAlerts      `json:"blockOnAlerts"`
			}{},
			ReleaseChannel:           "",
			AllowExperimentalModules: true,
		}, metricStorage),
		embeddedPolicy: helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
			Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
			ReleaseChannel: "Stable",
		}),
	}

	for _, option := range options {
		option(rec)
	}

	suite.r = rec
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha2.Module{},
			&v1alpha1.ModuleSource{},
			&v1alpha1.ModuleRelease{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ModuleSource"),
			v1alpha1.SchemeGroupVersion.WithKind("ModuleRelease"),
		},
		GoldenMode: reconcilertest.PerDocument,
	})
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && (testName == "TestCreateReconcile" || testName == "TestFetchMissingIntermediateReleases") {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
}

// TearDownSubTest only asserts the golden snapshot for the golden-driven test
// (TestCreateReconcile); the other tests make explicit assertions instead.
func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.compareGolden {
		suite.AssertGolden()
	}
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	got, err := reconcilertest.Snapshot(context.TODO(), suite.Client(), suite.Scheme(), reconcilertest.SnapshotSpec{
		Kinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ModuleSource"),
			v1alpha1.SchemeGroupVersion.WithKind("ModuleRelease"),
		},
	})
	require.NoError(suite.T(), err)

	return got
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("empty source", func() {
		suite.setupTestController("empty.yaml")
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "disabledmodule", "withpolicymodule", "notthissourcemodule", "bundlenabledmodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController("proceed-enabled-modules.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules without default", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "notthissourcemodule", "bundlenabledmodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController("proceed-enabled-modules-without-default.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("source with pull error", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{"enabledmodule", "errormodule"}, nil)
		dependency.TestDC.CRClient.ImageMock.Set(func(_ context.Context, tag string) (crv1.Image, error) {
			if tag == "alpha" {
				return nil, errors.New("GET https://registry.deckhouse.io/v2/deckhouse/ee/modules/errormodule/release/manifests/alpha:\n      MANIFEST_UNKNOWN: manifest unknown; map[Tag:alpha]")
			}

			return &crfake.FakeImage{
				ManifestStub: manifestStub,
				LayersStub: func() ([]crv1.Layer, error) {
					return []crv1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.2.3"}`}}}, nil
				},
				DigestStub: func() (crv1.Hash, error) {
					return crv1.Hash{Algorithm: "sha256"}, nil
				},
			}, nil
		})

		suite.setupTestController("module-pull-error.yaml")
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules with old version in module", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "disabledmodule", "withpolicymodule", "notthissourcemodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController("proceed-enabled-modules-with-old-version.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("module source without module releases", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.4.2",
			[]string{"enabledmodule"},
			[]string{})
		suite.setupTestController("without-module-releases.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("module source with existing module releases apply last patch", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.4.4",
			[]string{"parca"},
			[]string{"v1.4.1", "v1.4.2", "v1.4.3", "v1.4.4"},
		)
		suite.setupTestController("existing-module-releases-without-listing-registry.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("source with module releases and registry check", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.7.1",
			[]string{"parca"},
			[]string{"v1.3.1", "v1.4.1", "v1.5.2", "v1.5.3", "v1.6.1", "v1.6.2", "v1.7.1", "v1.7.2"})
		suite.setupTestController("existing-module-releases-with-listing-registry.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("LTS channel module minor version jump +20", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v0.25.0",
			[]string{"testmodule"},
			[]string{"v0.5.0", "v0.25.0"})
		suite.setupTestController("module-lts-channel-minor-jump.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)

		// Check that LTS channel creates direct update to latest version, skipping intermediates
		releases := suite.fetchResults()
		releasesStr := string(releases)

		// Should contain the target version
		assert.Contains(suite.T(), releasesStr, "testmodule-v0.25.0")
		// Should contain the deployed version
		assert.Contains(suite.T(), releasesStr, "testmodule-v0.5.0")
	})

	suite.Run("LTS channel module major version jump +1", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.0.0",
			[]string{"testmodule"},
			[]string{"v0.8.0", "v1.0.0"})
		suite.setupTestController("module-lts-channel-major-jump.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)

		// Check that LTS channel creates direct update to latest version, skipping intermediates
		releases := suite.fetchResults()
		releasesStr := string(releases)

		// Should contain the target version
		assert.Contains(suite.T(), releasesStr, "testmodule-v1.0.0")
		// Should contain the deployed version
		assert.Contains(suite.T(), releasesStr, "testmodule-v0.8.0")
	})

	suite.Run("LTS channel module multiple versions - should create only latest", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v0.7.0",
			[]string{"testmodule"},
			[]string{"v0.3.0", "v0.5.0", "v0.7.0"})
		suite.setupTestController("module-lts-channel-multiple-versions.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)

		// Check that LTS channel creates only the latest version, skipping intermediate
		releases := suite.fetchResults()
		releasesStr := string(releases)

		// Should contain the latest version
		assert.Contains(suite.T(), releasesStr, "testmodule-v0.7.0")
		// Should contain the deployed version
		assert.Contains(suite.T(), releasesStr, "testmodule-v0.3.0")
		// Should NOT contain intermediate version
		assert.NotContains(suite.T(), releasesStr, "testmodule-v0.5.0")
	})

	// A module that is still embedded but already published in an external source is
	// pre-staged: a single source resolves automatically, so a ModuleRelease is created.
	suite.Run("embedded module with a single source", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-single-source.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})

	// Several sources offer the embedded module and none is chosen via ModuleConfig:
	// it is a conflict, so no ModuleRelease is created.
	suite.Run("embedded module with several sources and no choice", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-several-sources-conflict.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})

	// The operator pinned a source via ModuleConfig that does not offer the module
	// (a stale or mistyped .spec.source - e.g. the source stopped publishing the
	// module after the config was admitted). It must be treated as a conflict, not
	// silently skipped: no ModuleRelease is created and the conflict alert fires.
	suite.Run("embedded module with a chosen source that is not available", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-stale-chosen-source.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})

	// Several sources, but the operator pinned the reconciled source via ModuleConfig:
	// a ModuleRelease is created from the chosen source.
	suite.Run("embedded module with several sources and a chosen source", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-chosen-source.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})

	// Several sources and ModuleConfig pins a different source than the reconciled one:
	// the reconciled source must not pre-stage a release.
	suite.Run("embedded module with several sources and a different chosen source", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-other-chosen-source.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})

	// "Embedded" is the sentinel for the built-in copy, not a real source, so a
	// ModuleConfig with source: Embedded is treated as "no choice" - several sources
	// remain a conflict and no ModuleRelease is created.
	suite.Run("embedded module with several sources and Embedded chosen source", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"ingressnginx"},
			[]string{})
		suite.setupTestController("embedded-module-embedded-chosen-source.yaml", withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	})
}

// TestFetchMissingIntermediateReleases reproduces the frozen-chain bug: the target
// release already exists and its checksum matches the one recorded on the source (so
// the plain checksum guard would skip the fetch), yet the step-by-step chain from the
// deployed release up to the target has a gap because the intermediate versions were
// mirrored into the registry only after the target release was first created. The
// reconcile must re-derive the chain and create the missing intermediate releases.
func (suite *ControllerTestSuite) TestFetchMissingIntermediateReleases() {
	suite.Run("frozen chain with missing intermediates is re-derived", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.55.1",
			[]string{"console"},
			[]string{"v1.49.1", "v1.50.0", "v1.51.1", "v1.52.0", "v1.53.2", "v1.54.1", "v1.55.1"})
		suite.setupTestController("frozen-chain-missing-intermediates.yaml", withDependencyContainer(dc))

		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
		// the resulting ModuleReleases (including the newly created console-v1.53.2 and
		// console-v1.54.1) are asserted against the golden snapshot in TearDownSubTest
	})
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("source with finalizer and empty releases", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`
		suite.setupTestControllerRaw(m)

		result, err := suite.r.deleteModuleSource(context.TODO(), suite.moduleSource("test-source"))
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		require.NoError(suite.T(), err)
		assert.Len(suite.T(), suite.moduleSource("test-source").Finalizers, 0)
	})

	suite.Run("source with finalizer and release", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-2
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-2
    status: deployed
  name: some-module-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-2
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`
		suite.setupTestControllerRaw(m)

		result, err := suite.r.deleteModuleSource(context.TODO(), suite.moduleSource("test-source-2"))
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Equal(suite.T(), 5*time.Second, result.RequeueAfter)

		source := suite.moduleSource("test-source-2")
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), source.Finalizers, 1)
		assert.Equal(suite.T(), source.Status.Message, "The source contains at least 1 deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue")
	})

	suite.Run("source with finalizer, annotation and release", func() {
		m := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-3
  annotations:
    modules.deckhouse.io/force-delete: "true"
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module-2
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-3
    status: deployed
  name: some-module-2-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-3
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module-2
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`
		suite.setupTestControllerRaw(m)

		result, err := suite.r.deleteModuleSource(context.TODO(), suite.moduleSource("test-source-3"))
		require.NoError(suite.T(), err)
		assert.False(suite.T(), result.Requeue)
		assert.Empty(suite.T(), result.RequeueAfter)

		assert.Len(suite.T(), suite.moduleSource("test-source-3").Finalizers, 0)
	})
}

func (suite *ControllerTestSuite) TestInvalidRegistry() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "false")
	invalidSource := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`
	suite.setupTestControllerRaw(invalidSource)

	_, err := suite.r.handleModuleSource(context.Background(), suite.moduleSource("test-source"))
	require.NoError(suite.T(), err)

	source := suite.moduleSource("test-source")
	assert.Contains(suite.T(), source.Status.Message, "credentials not found in the dockerCfg")
	assert.Len(suite.T(), source.Status.AvailableModules, 0)
}

// A module some source offers and nothing installed has an object of its own.
func (suite *ControllerTestSuite) TestAvailableModules() {
	const firstSource = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 912e02634dd8b7222cc42906e35f1e79
  name: test-source-1
spec:
  scanInterval: 6h30m
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`

	// the second source lists the module already, the first one pulls it in the scan under test
	const secondSource = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 912e02634dd8b7222cc42906e35f1e79
  name: test-source-2
spec:
  scanInterval: 6h30m
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
status:
  modules:
  - name: available
`

	scan := func(modules ...string) {
		dc := newMockedContainerWithData(suite.T(), "v1.2.3", modules, []string{})
		suite.r.dc = dc
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)
	}

	suite.Run("a single source offers the module", func() {
		suite.setupTestControllerRaw(firstSource)
		scan("available")

		module := suite.module("available")
		assert.Equal(suite.T(), "test-source-1", module.Spec.PackageRepositoryName)
		assert.Empty(suite.T(), module.Spec.PackageVersion)
		assert.Equal(suite.T(), "Stable", module.Spec.ReleaseChannel, "the channel of the embedded policy")
		assert.Equal(suite.T(), v1alpha1.ModulePhaseAvailable, module.Status.Phase)
		assert.Equal(suite.T(), v1alpha1.ModuleReasonNotInstalled, conditionReason(module, v1alpha1.ModuleConditionIsReady))
		assert.True(suite.T(), module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, "False"))
		assert.Empty(suite.T(), suite.releases().Items, "a module nothing enabled fetches no release")
	})

	suite.Run("several sources offer the module and the config picks none", func() {
		suite.setupTestControllerRaw(firstSource + secondSource + `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: available
spec:
  enabled: true
`)
		scan("available")

		module := suite.module("available")
		assert.Empty(suite.T(), module.Spec.PackageRepositoryName, "no source is picked")
		assert.Equal(suite.T(), v1alpha1.ModulePhaseConflict, module.Status.Phase)
		assert.Equal(suite.T(), v1alpha1.ModuleReasonConflict, conditionReason(module, v1alpha1.ModuleConditionIsReady))
		assert.Empty(suite.T(), suite.releases().Items, "no source installs a module in conflict")
	})

	suite.Run("several sources offer the module and the config picks one", func() {
		suite.setupTestControllerRaw(firstSource + secondSource + `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: available
spec:
  enabled: true
  source: test-source-2
`)
		scan("available")

		module := suite.module("available")
		assert.Equal(suite.T(), "test-source-2", module.Spec.PackageRepositoryName)
		assert.Equal(suite.T(), v1alpha1.ModulePhaseAvailable, module.Status.Phase)
		assert.Empty(suite.T(), suite.releases().Items, "the other source installs the module")
	})

	suite.Run("an enabled module fetches its first release", func() {
		suite.setupTestControllerRaw(firstSource + `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: available
spec:
  enabled: true
`)
		scan("available")

		module := suite.module("available")
		assert.Equal(suite.T(), "test-source-1", module.Spec.PackageRepositoryName)
		assert.Empty(suite.T(), module.Spec.PackageVersion, "the deploy fills the version")
		assert.Equal(suite.T(), v1alpha1.ModulePhaseDownloading, module.Status.Phase)
		assert.NotEmpty(suite.T(), suite.releases().Items)
	})

	suite.Run("an installed module keeps its placement when another source offers it", func() {
		suite.setupTestControllerRaw(firstSource + `
---
apiVersion: deckhouse.io/v1alpha2
kind: Module
metadata:
  name: available
spec:
  packageRepositoryName: test-source-2
  packageVersion: v1.0.0
  releaseChannel: Alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: available
spec:
  enabled: true
`)
		scan("available")

		module := suite.module("available")
		assert.Equal(suite.T(), "test-source-2", module.Spec.PackageRepositoryName)
		assert.Equal(suite.T(), "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(suite.T(), "Alpha", module.Spec.ReleaseChannel)
		assert.Empty(suite.T(), module.Status.Phase, "the not-installed state belongs to a module nothing installed")
		assert.Empty(suite.T(), suite.releases().Items, "the module comes from the other source")
	})
}

// A source that stops offering a module, or goes away, takes itself away from the module's
// object; the object of a module nothing installed and no other source offers goes.
func (suite *ControllerTestSuite) TestAvailableCleanup() {
	const sources = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 912e02634dd8b7222cc42906e35f1e79
  name: test-source-1
  finalizers:
  - modules.deckhouse.io/module-exists
spec:
  scanInterval: 6h30m
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
status:
  modules:
  - name: gone
  - name: shared
  - name: installed
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 912e02634dd8b7222cc42906e35f1e79
  name: test-source-2
spec:
  scanInterval: 6h30m
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
status:
  modules:
  - name: shared
---
apiVersion: deckhouse.io/v1alpha2
kind: Module
metadata:
  name: gone
spec:
  packageRepositoryName: test-source-1
  releaseChannel: Stable
status:
  phase: Available
---
apiVersion: deckhouse.io/v1alpha2
kind: Module
metadata:
  name: shared
spec:
  releaseChannel: Stable
status:
  phase: Available
---
apiVersion: deckhouse.io/v1alpha2
kind: Module
metadata:
  name: installed
spec:
  packageRepositoryName: test-source-1
  packageVersion: v1.0.0
  releaseChannel: Stable
`

	assertCleaned := func() {
		err := suite.Client().Get(context.TODO(), types.NamespacedName{Name: "gone"}, new(v1alpha2.Module))
		assert.True(suite.T(), apierrors.IsNotFound(err), "no source offers the module any more")

		shared := suite.module("shared")
		assert.Equal(suite.T(), "test-source-2", shared.Spec.PackageRepositoryName, "the remaining source places the module")
		assert.Equal(suite.T(), v1alpha1.ModulePhaseAvailable, shared.Status.Phase)

		installed := suite.module("installed")
		assert.Equal(suite.T(), "test-source-1", installed.Spec.PackageRepositoryName, "an installed module is not touched")
		assert.Equal(suite.T(), "v1.0.0", installed.Spec.PackageVersion)
	}

	suite.Run("the source stops offering the modules", func() {
		suite.setupTestControllerRaw(sources)

		dc := newMockedContainerWithData(suite.T(), "v1.2.3", []string{}, []string{})
		suite.r.dc = dc
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)

		assertCleaned()
	})

	suite.Run("the source is deleted", func() {
		suite.setupTestControllerRaw(sources)

		_, err := suite.r.deleteModuleSource(context.TODO(), suite.moduleSource("test-source-1"))
		require.NoError(suite.T(), err)

		assertCleaned()
		assert.Empty(suite.T(), suite.moduleSource("test-source-1").Finalizers)
	})
}

func (suite *ControllerTestSuite) module(name string) *v1alpha2.Module {
	module := new(v1alpha2.Module)
	require.NoError(suite.T(), suite.Client().Get(context.TODO(), types.NamespacedName{Name: name}, module))

	return module
}

func (suite *ControllerTestSuite) releases() *v1alpha1.ModuleReleaseList {
	releases := new(v1alpha1.ModuleReleaseList)
	require.NoError(suite.T(), suite.Client().List(context.TODO(), releases))

	return releases
}

func conditionReason(module *v1alpha2.Module, condType string) string {
	for _, cond := range module.Status.Conditions {
		if cond.Type == condType {
			return cond.Reason
		}
	}

	return ""
}

func (suite *ControllerTestSuite) moduleSource(name string) *v1alpha1.ModuleSource {
	source := new(v1alpha1.ModuleSource)
	err := suite.Client().Get(context.TODO(), types.NamespacedName{Name: name}, source)
	require.NoError(suite.T(), err)

	return source
}

func newMockedContainerWithData(t minimock.Tester, versionInChannel string, modules, tags []string) *dependency.MockedContainer {
	dc := dependency.NewMockedContainer()

	dc.CRClientMap = map[string]cr.Client{
		"dev-registry.deckhouse.io/deckhouse/modules": cr.NewClientMock(t).ListTagsMock.Return(modules, nil),
	}

	for _, module := range modules {
		moduleVersionsMock := cr.NewClientMock(t)

		if len(tags) > 0 {
			dc.CRClientMap["dev-registry.deckhouse.io/deckhouse/modules/"+module] = moduleVersionsMock.ListTagsMock.Optional().Return(tags, nil)
		}

		dc.CRClientMap["dev-registry.deckhouse.io/deckhouse/modules/"+module+"/release"] = moduleVersionsMock.ImageMock.Optional().Set(func(_ context.Context, imageTag string) (crv1.Image, error) {
			_, err := semver.NewVersion(imageTag)
			if err != nil {
				imageTag = versionInChannel
			}

			moduleYaml := `
name: ` + module + `
weight: 900
stage: "General Availability"
requirements:
  kubernetes: ">= 1.27"
disable:
  confirmation: true
  message: "Disabling this module will completely stop normal operation of the Deckhouse Kubernetes Platform."
`

			if module == "bundlenabledmodule" {
				moduleYaml += `
accessibility:
   editions:
      fe:
         available: true
         enabledInBundles:
            - Default
`
			}

			return &crfake.FakeImage{
				ManifestStub: manifestStub,
				LayersStub: func() ([]crv1.Layer, error) {
					return []crv1.Layer{
						&utils.FakeLayer{},
						&utils.FakeLayer{FilesContent: map[string]string{
							"module.yaml":  moduleYaml,
							"version.json": `{"version": "` + imageTag + `"}`,
						}},
					}, nil
				},
				DigestStub: func() (crv1.Hash, error) {
					return crv1.Hash{Algorithm: "sha256"}, nil
				},
			}, nil
		})
	}

	dc.CRClient.ListTagsMock.Return(modules, nil)

	dc.CRClient.ImageMock.Return(&crfake.FakeImage{
		ManifestStub: manifestStub,
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{}, &utils.FakeLayer{FilesContent: map[string]string{"version.json": `{"version": "` + versionInChannel + `"}`}}}, nil
		},
		DigestStub: func() (crv1.Hash, error) {
			return crv1.Hash{Algorithm: "sha256"}, nil
		},
	}, nil)

	return dc
}

func (suite *ControllerTestSuite) TestFilterInvalidModuleNames() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "false")

	sourceYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
spec:
  registry:
    dockerCfg: ""
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
`

	suite.setupTestControllerRaw(sourceYAML)

	pulledModules := []string{
		"modules",               // reserved
		strings.Repeat("a", 65), // too big
		"invalid_name!",         // invalid RFC1123
		"Cloud-Provider-AWS",    // invalid RFC1123
		"-invalid-module",       // invalid RFC1123
		"invalid_module",        // invalid RFC1123
		"valid.module",          //	ok
		"valid-module",          // ok
		"another-valid-module",  // ok
	}

	err := suite.r.processModules(context.Background(), suite.moduleSource("test-source"), nil, pulledModules)
	require.NoError(suite.T(), err)

	source := suite.moduleSource("test-source")

	moduleNames := make([]string, 0, len(source.Status.AvailableModules))
	for _, mod := range source.Status.AvailableModules {
		moduleNames = append(moduleNames, mod.Name)
	}

	assert.ElementsMatch(suite.T(), []string{"valid-module", "valid.module", "another-valid-module"}, moduleNames)
}
