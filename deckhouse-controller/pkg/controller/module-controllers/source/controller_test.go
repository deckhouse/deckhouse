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
	"fmt"
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
			&v1alpha1.Module{},
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
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
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

// TestUpdateModulePropertiesFromRegistry tests that modules in Phase: Available
// get their properties updated from registry when new version with different metadata appears.
//
// Scenario:
// 1. Module exists with Phase: Available, Stage: Experimental
// 2. New version in registry has Stage: Stable
// 3. After processModules, Module.Properties.Stage should be updated to Stable
func (suite *ControllerTestSuite) TestUpdateModulePropertiesFromRegistry() {
	suite.Run("available module updates all properties from registry", func() {
		// Initial state: module with Phase: Available and old properties
		initialManifest := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 90f0955ee984feab5c50611987008def
    modules.deckhouse.io/default-source: "true"
  name: test-source
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: testmodule
properties:
  source: test-source
  stage: Experimental
  weight: 900
  namespace: d8-testmodule-old
  version: v1.0.0
  critical: false
  subsystems:
    - old-subsystem
  exclusiveGroup: old-group
  availableSources:
    - test-source
status:
  phase: Available
  conditions:
    - type: EnabledByModuleConfig
      status: "False"
`
		// Mock registry to return new version with ALL properties changed
		dc := newMockedContainerWithModuleDefinition(suite.T(),
			"v1.1.0",
			[]string{"testmodule"},
			[]string{},
			map[string]moduleDefinitionConfig{
				"testmodule": {
					stage:          "Stable",
					weight:         850,
					namespace:      "d8-testmodule-new",
					critical:       true,
					subsystems:     []string{"security", "monitoring"},
					exclusiveGroup: "new-exclusive-group",
					requirements: &v1alpha1.ModuleRequirements{
						ModulePlatformRequirements: v1alpha1.ModulePlatformRequirements{
							Deckhouse:  ">= 1.60",
							Kubernetes: ">= 1.28",
						},
					},
					disableOptions: &v1alpha1.ModuleDisableOptions{
						Confirmation: true,
						Message:      "This module is critical for cluster operation",
					},
					accessibility: map[string]struct {
						available        bool
						enabledInBundles []string
					}{
						"fe": {
							available:        true,
							enabledInBundles: []string{"Default", "Managed"},
						},
					},
				},
			},
		)

		suite.setupTestControllerRaw(initialManifest, withDependencyContainer(dc))

		// Execute
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source"))
		require.NoError(suite.T(), err)

		// Verify: ALL module properties should be updated from registry
		module := suite.module("testmodule")

		// Properties from updateModulePropertiesFromDefinition:
		// props.Stage = def.Stage
		assert.Equal(suite.T(), "Stable", module.Properties.Stage, "Stage should be updated")

		// props.Weight = def.Weight
		assert.Equal(suite.T(), uint32(850), module.Properties.Weight, "Weight should be updated")

		// props.Critical = def.Critical
		assert.Equal(suite.T(), true, module.Properties.Critical, "Critical should be updated")

		// props.Namespace = def.Namespace
		assert.Equal(suite.T(), "d8-testmodule-new", module.Properties.Namespace, "Namespace should be updated")

		// props.Subsystems = def.Subsystems
		assert.Equal(suite.T(), []string{"security", "monitoring"}, module.Properties.Subsystems, "Subsystems should be updated")

		// props.ExclusiveGroup = def.ExclusiveGroup
		assert.Equal(suite.T(), "new-exclusive-group", module.Properties.ExclusiveGroup, "ExclusiveGroup should be updated")

		// props.Requirements = def.Requirements
		require.NotNil(suite.T(), module.Properties.Requirements, "Requirements should not be nil")
		assert.Equal(suite.T(), ">= 1.60", module.Properties.Requirements.Deckhouse, "Requirements.Deckhouse should be updated")
		assert.Equal(suite.T(), ">= 1.28", module.Properties.Requirements.Kubernetes, "Requirements.Kubernetes should be updated")

		// props.DisableOptions = def.DisableOptions
		require.NotNil(suite.T(), module.Properties.DisableOptions, "DisableOptions should not be nil")
		assert.Equal(suite.T(), true, module.Properties.DisableOptions.Confirmation, "DisableOptions.Confirmation should be updated")
		assert.Equal(suite.T(), "This module is critical for cluster operation", module.Properties.DisableOptions.Message, "DisableOptions.Message should be updated")

		// props.Accessibility = def.Accessibility.ToV1Alpha1()
		require.NotNil(suite.T(), module.Properties.Accessibility, "Accessibility should not be nil")
		require.Contains(suite.T(), module.Properties.Accessibility.Editions, "fe", "Accessibility should contain 'fe' edition")
		assert.Equal(suite.T(), true, module.Properties.Accessibility.Editions["fe"].Available, "Accessibility.Editions[fe].Available should be true")
		assert.Equal(suite.T(), []string{"Default", "Managed"}, module.Properties.Accessibility.Editions["fe"].EnabledInBundles, "Accessibility.Editions[fe].EnabledInBundles should be updated")

		// props.Version = moduleVersion
		assert.Equal(suite.T(), "v1.1.0", module.Properties.Version, "Version should be updated")
	})

	suite.Run("non-available module does not update properties from registry", func() {
		// Initial state: module with Phase: Ready (installed) - properties should NOT be updated
		initialManifest := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    modules.deckhouse.io/registry-spec-checksum: 90f0955ee984feab5c50611987008def
    modules.deckhouse.io/default-source: "true"
  name: test-source
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: installedmodule
properties:
  source: test-source
  stage: Experimental
  weight: 900
  namespace: d8-installedmodule
  version: v1.0.0
  critical: false
  subsystems:
    - old-subsystem
  exclusiveGroup: old-group
  availableSources:
    - test-source
status:
  phase: Ready
  conditions:
    - type: EnabledByModuleConfig
      status: "True"
    - type: EnabledByModuleManager
      status: "True"
`
		// Mock registry to return new version with ALL properties changed
		dc := newMockedContainerWithModuleDefinition(suite.T(),
			"v1.1.0",
			[]string{"installedmodule"},
			[]string{},
			map[string]moduleDefinitionConfig{
				"installedmodule": {
					stage:          "Stable",
					weight:         850,
					namespace:      "d8-installedmodule-new",
					critical:       true,
					subsystems:     []string{"security", "monitoring"},
					exclusiveGroup: "new-exclusive-group",
					requirements: &v1alpha1.ModuleRequirements{
						ModulePlatformRequirements: v1alpha1.ModulePlatformRequirements{
							Deckhouse:  ">= 1.60",
							Kubernetes: ">= 1.28",
						},
					},
					disableOptions: &v1alpha1.ModuleDisableOptions{
						Confirmation: true,
						Message:      "This module is critical",
					},
					accessibility: map[string]struct {
						available        bool
						enabledInBundles []string
					}{
						"fe": {
							available:        true,
							enabledInBundles: []string{"Default"},
						},
					},
				},
			},
		)

		suite.setupTestControllerRaw(initialManifest, withDependencyContainer(dc))

		// Execute
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource("test-source"))
		require.NoError(suite.T(), err)

		// Verify: ALL module properties should NOT be updated (Phase != Available)
		module := suite.module("installedmodule")

		// All properties should remain unchanged:
		assert.Equal(suite.T(), "Experimental", module.Properties.Stage, "Stage should NOT be updated for installed module")
		assert.Equal(suite.T(), uint32(900), module.Properties.Weight, "Weight should NOT be updated for installed module")
		assert.Equal(suite.T(), false, module.Properties.Critical, "Critical should NOT be updated for installed module")
		assert.Equal(suite.T(), "d8-installedmodule", module.Properties.Namespace, "Namespace should NOT be updated for installed module")
		assert.Equal(suite.T(), []string{"old-subsystem"}, module.Properties.Subsystems, "Subsystems should NOT be updated for installed module")
		assert.Equal(suite.T(), "old-group", module.Properties.ExclusiveGroup, "ExclusiveGroup should NOT be updated for installed module")
		assert.Nil(suite.T(), module.Properties.Requirements, "Requirements should remain nil for installed module")
		assert.Nil(suite.T(), module.Properties.DisableOptions, "DisableOptions should remain nil for installed module")
		assert.Nil(suite.T(), module.Properties.Accessibility, "Accessibility should remain nil for installed module")
		assert.Equal(suite.T(), "v1.0.0", module.Properties.Version, "Version should NOT be updated for installed module")
	})
}

// moduleDefinitionConfig holds configuration for mocking module definition.
// All fields correspond to properties updated in updateModulePropertiesFromDefinition.
type moduleDefinitionConfig struct {
	stage          string
	weight         uint32
	namespace      string
	critical       bool
	subsystems     []string
	exclusiveGroup string
	requirements   *v1alpha1.ModuleRequirements
	disableOptions *v1alpha1.ModuleDisableOptions
	accessibility  map[string]struct {
		available        bool
		enabledInBundles []string
	}
}

// newMockedContainerWithModuleDefinition creates a mocked container that returns
// specific module definitions for each module
func newMockedContainerWithModuleDefinition(
	t minimock.Tester,
	versionInChannel string,
	modules, tags []string,
	definitions map[string]moduleDefinitionConfig,
) *dependency.MockedContainer {
	dc := dependency.NewMockedContainer()

	dc.CRClientMap = map[string]cr.Client{
		"dev-registry.deckhouse.io/deckhouse/modules": cr.NewClientMock(t).ListTagsMock.Return(modules, nil),
	}

	for _, module := range modules {
		moduleVersionsMock := cr.NewClientMock(t)

		if len(tags) > 0 {
			dc.CRClientMap["dev-registry.deckhouse.io/deckhouse/modules/"+module] = moduleVersionsMock.ListTagsMock.Optional().Return(tags, nil)
		}

		def, ok := definitions[module]
		if !ok {
			def = moduleDefinitionConfig{
				stage:     "General Availability",
				weight:    900,
				namespace: "d8-" + module,
				critical:  false,
			}
		}

		// Capture def for closure
		moduleDef := def
		moduleName := module

		dc.CRClientMap["dev-registry.deckhouse.io/deckhouse/modules/"+module+"/release"] = moduleVersionsMock.ImageMock.Optional().Set(func(_ context.Context, imageTag string) (crv1.Image, error) {
			_, err := semver.NewVersion(imageTag)
			if err != nil {
				imageTag = versionInChannel
			}

			moduleYaml := buildModuleYaml(moduleName, moduleDef)

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
					return crv1.Hash{Algorithm: "sha256", Hex: "abc123"}, nil
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
			return crv1.Hash{Algorithm: "sha256", Hex: "abc123"}, nil
		},
	}, nil)

	return dc
}

// module helper to get module by name
func (suite *ControllerTestSuite) module(name string) *v1alpha1.Module {
	module := new(v1alpha1.Module)
	err := suite.Client().Get(context.TODO(), types.NamespacedName{Name: name}, module)
	require.NoError(suite.T(), err)

	return module
}

// buildModuleYaml generates module.yaml content from moduleDefinitionConfig
func buildModuleYaml(moduleName string, def moduleDefinitionConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("name: %s\n", moduleName))
	sb.WriteString(fmt.Sprintf("weight: %d\n", def.weight))
	sb.WriteString(fmt.Sprintf("stage: \"%s\"\n", def.stage))
	sb.WriteString(fmt.Sprintf("namespace: %s\n", def.namespace))
	sb.WriteString(fmt.Sprintf("critical: %t\n", def.critical))

	if len(def.subsystems) > 0 {
		sb.WriteString("subsystems:\n")
		for _, s := range def.subsystems {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	if def.exclusiveGroup != "" {
		sb.WriteString(fmt.Sprintf("exclusiveGroup: %s\n", def.exclusiveGroup))
	}

	if def.requirements != nil {
		sb.WriteString("requirements:\n")
		if def.requirements.Deckhouse != "" {
			sb.WriteString(fmt.Sprintf("  deckhouse: \"%s\"\n", def.requirements.Deckhouse))
		}
		if def.requirements.Kubernetes != "" {
			sb.WriteString(fmt.Sprintf("  kubernetes: \"%s\"\n", def.requirements.Kubernetes))
		}
	}

	if def.disableOptions != nil {
		sb.WriteString("disable:\n")
		sb.WriteString(fmt.Sprintf("  confirmation: %t\n", def.disableOptions.Confirmation))
		if def.disableOptions.Message != "" {
			sb.WriteString(fmt.Sprintf("  message: \"%s\"\n", def.disableOptions.Message))
		}
	}

	if def.accessibility != nil {
		sb.WriteString("accessibility:\n")
		sb.WriteString("  editions:\n")
		for editionName, edition := range def.accessibility {
			sb.WriteString(fmt.Sprintf("    %s:\n", editionName))
			sb.WriteString(fmt.Sprintf("      available: %t\n", edition.available))
			if len(edition.enabledInBundles) > 0 {
				sb.WriteString("      enabledInBundles:\n")
				for _, bundle := range edition.enabledInBundles {
					sb.WriteString(fmt.Sprintf("        - %s\n", bundle))
				}
			}
		}
	}

	return sb.String()
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
