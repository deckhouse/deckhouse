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
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
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
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

var (
	generateGolden     bool
	manifestsDelimiter *regexp.Regexp
	manifestStub       = func() (*crv1.Manifest, error) {
		return &crv1.Manifest{
			Layers: []crv1.Descriptor{},
		}, nil
	}
)

func init() {
	flag.BoolVar(&generateGolden, "golden", false, "generate golden files")
	manifestsDelimiter = regexp.MustCompile("(?m)^---$")
}

type ControllerTestSuite struct {
	suite.Suite

	client client.Client
	r      *reconciler

	goldenFile    string
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

func (suite *ControllerTestSuite) setupTestController(raw string, options ...reconcilerOption) {
	manifests := releaseutil.SplitManifests(raw)

	var objects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.parseKubernetesObject([]byte(manifest))
		objects = append(objects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = v1alpha2.SchemeBuilder.AddToScheme(sc)
	suite.client = fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.Module{}, &v1alpha1.ModuleSource{}, &v1alpha1.ModuleRelease{}).
		Build()

	rec := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               suite.client,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dependency.NewDependencyContainer(),
		logger:               log.NewNop(),
		edition: &d8edition.Edition{
			Name:   "fe",
			Bundle: "Default",
		},
		metricStorage: metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop())),

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

func (suite *ControllerTestSuite) parseKubernetesObject(raw []byte) client.Object {
	metaType := new(runtime.TypeMeta)
	err := yaml.Unmarshal(raw, metaType)
	require.NoError(suite.T(), err)

	var obj client.Object

	switch metaType.Kind {
	case v1alpha1.ModuleSourceGVK.Kind:
		source := new(v1alpha1.ModuleSource)
		err = yaml.Unmarshal(raw, source)
		require.NoError(suite.T(), err)
		obj = source
		suite.source = source.Name

	case v1alpha1.ModuleReleaseGVK.Kind:
		release := new(v1alpha1.ModuleRelease)
		err = yaml.Unmarshal(raw, release)
		require.NoError(suite.T(), err)
		obj = release

	case v1alpha2.ModuleUpdatePolicyGVK.Kind:
		policy := new(v1alpha2.ModuleUpdatePolicy)
		err = yaml.Unmarshal(raw, policy)
		require.NoError(suite.T(), err)
		obj = policy

	case v1alpha1.ModuleGVK.Kind:
		module := new(v1alpha1.Module)
		err = yaml.Unmarshal(raw, module)
		require.NoError(suite.T(), err)
		obj = module
	}

	return obj
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
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

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}

	currentObjects := suite.fetchResults()

	if generateGolden {
		err := os.WriteFile(suite.goldenFile, currentObjects, 0666)
		require.NoError(suite.T(), err)
		return
	}

	raw, err := os.ReadFile(suite.goldenFile)
	require.NoError(suite.T(), err)

	exp := splitManifests(raw)
	got := splitManifests(currentObjects)

	require.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
	for i := range got {
		assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
	}
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	sources := new(v1alpha1.ModuleSourceList)
	err := suite.client.List(context.TODO(), sources)
	require.NoError(suite.T(), err)

	for _, source := range sources.Items {
		got, _ := yaml.Marshal(source)
		result.WriteString("---\n")
		result.Write(got)
	}

	releases := new(v1alpha1.ModuleReleaseList)
	err = suite.client.List(context.TODO(), releases)
	require.NoError(suite.T(), err)

	for _, release := range releases.Items {
		got, _ := yaml.Marshal(release)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func splitManifests(doc []byte) []string {
	splits := manifestsDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(splits))
	for i := range splits {
		if splits[i] != "" {
			result = append(result, splits[i])
		}
	}

	return result
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("empty source", func() {
		suite.setupTestController(string(suite.parseTestdata("empty.yaml")))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "disabledmodule", "withpolicymodule", "notthissourcemodule", "bundlenabledmodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController(string(suite.parseTestdata("proceed-enabled-modules.yaml")), withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules without default", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "notthissourcemodule", "bundlenabledmodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController(string(suite.parseTestdata("proceed-enabled-modules-without-default.yaml")), withDependencyContainer(dc))
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

		suite.setupTestController(string(suite.parseTestdata("module-pull-error.yaml")))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("proceed enabled modules with old version in module", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.2.3",
			[]string{"enabledmodule", "disabledmodule", "withpolicymodule", "notthissourcemodule"},
			// versions differ only in patch and we don't have requests to registry
			[]string{})
		suite.setupTestController(string(suite.parseTestdata("proceed-enabled-modules-with-old-version.yaml")), withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("module source without module releases", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.4.2",
			[]string{"enabledmodule"},
			[]string{})
		suite.setupTestController(string(suite.parseTestdata("without-module-releases.yaml")), withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("module source with existing module releases apply last patch without listing tags", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.4.4",
			[]string{"parca"},
			[]string{})
		suite.setupTestController(string(suite.parseTestdata("existing-module-releases-without-listing-registry.yaml")), withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("source with module releases and registry check", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v1.7.1",
			[]string{"parca"},
			[]string{"v1.3.1", "v1.4.1", "v1.5.2", "v1.5.3", "v1.6.1", "v1.6.2", "v1.7.1", "v1.7.2"})
		suite.setupTestController(string(suite.parseTestdata("existing-module-releases-with-listing-registry.yaml")), withDependencyContainer(dc))
		_, err := suite.r.handleModuleSource(context.TODO(), suite.moduleSource(suite.source))
		require.NoError(suite.T(), err)
	})

	suite.Run("LTS channel module minor version jump +20", func() {
		dc := newMockedContainerWithData(suite.T(),
			"v0.25.0",
			[]string{"testmodule"},
			[]string{"v0.5.0", "v0.25.0"})
		suite.setupTestController(string(suite.parseTestdata("module-lts-channel-minor-jump.yaml")), withDependencyContainer(dc))
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
		suite.setupTestController(string(suite.parseTestdata("module-lts-channel-major-jump.yaml")), withDependencyContainer(dc))
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
		suite.setupTestController(string(suite.parseTestdata("module-lts-channel-multiple-versions.yaml")), withDependencyContainer(dc))
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

func (suite *ControllerTestSuite) parseTestdata(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.goldenFile = filepath.Join("./testdata", "golden", filename)

	return data
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
		suite.setupTestController(m)

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
		suite.setupTestController(m)

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
		suite.setupTestController(m)

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
	suite.setupTestController(invalidSource)

	_, err := suite.r.handleModuleSource(context.Background(), suite.moduleSource("test-source"))
	require.NoError(suite.T(), err)

	source := suite.moduleSource("test-source")
	assert.Contains(suite.T(), source.Status.Message, "credentials not found in the dockerCfg")
	assert.Len(suite.T(), source.Status.AvailableModules, 0)
}

func (suite *ControllerTestSuite) moduleSource(name string) *v1alpha1.ModuleSource {
	source := new(v1alpha1.ModuleSource)
	err := suite.client.Get(context.TODO(), types.NamespacedName{Name: name}, source)
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

	suite.setupTestController(sourceYAML)

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
