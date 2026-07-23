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

package applicationpackageversion

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	golden     bool
	mDelimiter *regexp.Regexp
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	mDelimiter = regexp.MustCompile("(?m)^---$")
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

func TestSetPackageSchemaPreservesXUI(t *testing.T) {
	apv := new(v1alpha1.ApplicationPackageVersion)
	rawSchema := []byte(`
type: object
properties:
  basic:
    type: string
    x-deckhouse-ui-advanced: false
  expert:
    type: string
    x-deckhouse-ui-advanced: true
  unclassified:
    type: string
  groups:
    type: array
    items:
      type: string
    x-ui:
      label:
        en: Groups
        ru: Группы
      widget:
        name: ResourceSelect
        foreignResources:
          - name: groups.deckhouse.io
            source:
              optionValuePath: spec.name
        props:
          allowCreate: false
          filterable: true
          multiple: true
  legacyGroup:
    type: string
    deprecated: true
    x-ui:
      display: hide-field
`)

	require.NoError(t, setPackageSchema(apv, schemaTypeSettings, rawSchema))
	require.NotNil(t, apv.Status.PackageSchemas)
	require.NotNil(t, apv.Status.PackageSchemas.SettingsSchema)
	require.NotNil(t, apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema)

	groups := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["groups"]
	require.NotNil(t, groups.XUI)

	var ui map[string]any
	require.NoError(t, json.Unmarshal(groups.XUI.Raw, &ui))
	widget, ok := ui["widget"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ResourceSelect", widget["name"])
	props, ok := widget["props"].(map[string]any)
	require.True(t, ok)
	allowCreate, ok := props["allowCreate"].(bool)
	require.True(t, ok)
	require.False(t, allowCreate)
	foreignResources, ok := widget["foreignResources"].([]any)
	require.True(t, ok)
	require.Len(t, foreignResources, 1)
	resource, ok := foreignResources[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "groups.deckhouse.io", resource["name"])

	legacyGroup := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["legacyGroup"]
	require.True(t, legacyGroup.Deprecated)
	require.NotNil(t, legacyGroup.XUI)

	basic := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["basic"]
	require.NotNil(t, basic.XUIAdvanced)
	require.False(t, *basic.XUIAdvanced)
	expert := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["expert"]
	require.NotNil(t, expert.XUIAdvanced)
	require.True(t, *expert.XUIAdvanced)
	unclassified := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["unclassified"]
	require.Nil(t, unclassified.XUIAdvanced)
}

func TestPromotedAPVSchemaRehydration(t *testing.T) {
	t.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")

	t.Run("rehydrates once and then skips registry", func(t *testing.T) {
		ctr, kubeClient := setupFakeController(t, "missing-rehydration-testdata.yaml")
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Times(1).Return(packageImageWithSettings(), nil)
		ctr.dc = dc
		ctr.registry = registry.NewService(dc, log.NewNop())

		createRehydrationObjects(t, kubeClient, "")

		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "deckhouse-test-v1.0.0"}}
		_, err := ctr.Reconcile(context.Background(), request)
		require.NoError(t, err)

		apv := getAPV(t, kubeClient)
		require.Equal(t, currentPackageSchemaSerializationVersion, apv.Status.PackageSchemas.SerializationVersion)
		require.Equal(t, 1, apv.Status.UsedByCount)
		require.Equal(t, "demo", apv.Status.UsedBy[0].Name)
		require.Contains(t, apv.Finalizers, v1alpha1.ApplicationPackageVersionFinalizer)
		require.Equal(t, "preserved", apv.Labels["test.deckhouse.io/preserved"])

		root := apv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema
		require.NotNil(t, root.XUI)
		basic := root.Properties["mode"].XUIAdvanced
		require.NotNil(t, basic)
		require.False(t, *basic)

		_, err = ctr.Reconcile(context.Background(), request)
		require.NoError(t, err)
	})

	t.Run("current serialization does not read registry", func(t *testing.T) {
		ctr, kubeClient := setupFakeController(t, "missing-current-testdata.yaml")
		dc := dependency.NewMockedContainer()
		calls := 0
		dc.CRClient.ImageMock.Optional().Set(func(_ context.Context, _ string) (crv1.Image, error) {
			calls++
			return packageImageWithSettings(), nil
		})
		ctr.dc = dc
		ctr.registry = registry.NewService(dc, log.NewNop())

		createRehydrationObjects(t, kubeClient, currentPackageSchemaSerializationVersion)

		_, err := ctr.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "deckhouse-test-v1.0.0"},
		})
		require.NoError(t, err)
		require.Zero(t, calls)
	})

	t.Run("registry failure preserves existing status", func(t *testing.T) {
		ctr, kubeClient := setupFakeController(t, "missing-error-testdata.yaml")
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Times(1).Return(nil, fmt.Errorf("registry credential must not reach status"))
		ctr.dc = dc
		ctr.registry = registry.NewService(dc, log.NewNop())

		createRehydrationObjects(t, kubeClient, "")
		before := getAPV(t, kubeClient)

		_, err := ctr.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: before.Name},
		})
		require.Error(t, err)

		after := getAPV(t, kubeClient)
		require.Equal(t, before.Status.PackageMetadata, after.Status.PackageMetadata)
		require.Equal(t, before.Status.PackageSchemas, after.Status.PackageSchemas)
		require.Equal(t, before.Status.UsedBy, after.Status.UsedBy)
		condition := findCondition(after.Status.Conditions, v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded)
		require.NotNil(t, condition)
		require.Equal(t, metav1.ConditionFalse, condition.Status)
		require.Equal(t, "failed to rehydrate immutable package metadata; retrying", condition.Message)
	})
}

func packageImageWithSettings() crv1.Image {
	return &crfake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{Layers: []crv1.Descriptor{}}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{FilesContent: map[string]string{
				"package.yaml": `name: test
descriptions:
  en: Test package
  ru: Test package
stage: Preview
type: Application
version: "1.0.0"
`,
				"version.json": `{"version":"1.0.0"}`,
				"openapi/settings.yaml": `x-config-version: 1
type: object
x-ui:
  propertiesOrder:
    - mode
properties:
  mode:
    type: string
    x-deckhouse-ui-advanced: false
`,
			}}}, nil
		},
	}
}

func createRehydrationObjects(t *testing.T, kubeClient client.Client, serializationVersion string) {
	t.Helper()

	repo := &v1alpha1.PackageRepository{
		ObjectMeta: metav1.ObjectMeta{Name: "deckhouse"},
		Spec: v1alpha1.PackageRepositorySpec{
			Registry: v1alpha1.PackageRepositorySpecRegistry{
				Scheme:    "https",
				Repo:      "registry.example.com/test",
				DockerCFG: "test-docker-cfg",
			},
		},
	}
	require.NoError(t, kubeClient.Create(context.Background(), repo))

	apv := &v1alpha1.ApplicationPackageVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "deckhouse-test-v1.0.0",
			Labels:     map[string]string{"test.deckhouse.io/preserved": "preserved"},
			Finalizers: []string{v1alpha1.ApplicationPackageVersionFinalizer},
		},
		Spec: v1alpha1.ApplicationPackageVersionSpec{
			PackageName:           "test",
			PackageRepositoryName: "deckhouse",
			PackageVersion:        "v1.0.0",
		},
	}
	require.NoError(t, kubeClient.Create(context.Background(), apv))

	apv.Status = v1alpha1.ApplicationPackageVersionStatus{
		PackageMetadata: &v1alpha1.ApplicationPackageVersionStatusMetadata{
			Description: &v1alpha1.PackageDescription{En: "legacy"},
		},
		PackageSchemas: &v1alpha1.ApplicationPackageVersionStatusSchemas{
			SerializationVersion: serializationVersion,
			SettingsSchema: &v1alpha1.PackageSchema{
				OpenAPIV3Schema: &openapi.OpenAPIV3Schema{Type: "object"},
			},
		},
		UsedBy:      []v1alpha1.ApplicationPackageVersionStatusInstance{{Namespace: "demo", Name: "demo"}},
		UsedByCount: 1,
	}
	require.NoError(t, kubeClient.Status().Update(context.Background(), apv))
}

func getAPV(t *testing.T, kubeClient client.Client) *v1alpha1.ApplicationPackageVersion {
	t.Helper()

	apv := new(v1alpha1.ApplicationPackageVersion)
	require.NoError(t, kubeClient.Get(context.Background(), client.ObjectKey{Name: "deckhouse-test-v1.0.0"}, apv))
	return apv
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient       client.Client
	ctr              *reconciler
	testDataFileName string
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0o666)
		require.NoError(suite.T(), err)
	} else {
		got := singleDocToManifests(gotB)
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := singleDocToManifests(expB)
		assert.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
		for i := range got {
			assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
		}
	}
}

type reconcilerOption func(*reconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dc = dc
		r.registry = registry.NewService(dc, log.NewNop())
	}
}

func (suite *ControllerTestSuite) setupController(filename string, options ...reconcilerOption) {
	suite.testDataFileName = filename
	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), filename)

	for _, opt := range options {
		opt(suite.ctr)
	}
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var apvList v1alpha1.ApplicationPackageVersionList
	err := suite.kubeClient.List(context.TODO(), &apvList)
	require.NoError(suite.T(), err)

	for _, item := range apvList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var repoList v1alpha1.PackageRepositoryList
	err = suite.kubeClient.List(context.TODO(), &repoList)
	require.NoError(suite.T(), err)

	for _, item := range repoList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func singleDocToManifests(doc []byte) []string {
	split := mDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(split))
	for i := range split {
		if split[i] != "" {
			result = append(result, split[i])
		}
	}

	return result
}

func setupFakeController(t *testing.T, filename string) (*reconciler, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects().
		WithStatusSubresource(&v1alpha1.ApplicationPackageVersion{}).
		Build()

	dc := dependency.NewDependencyContainer()

	ctr := &reconciler{
		client:   kubeClient,
		logger:   log.NewNop(),
		dc:       dc,
		registry: registry.NewService(dc, log.NewNop()),
	}

	// Load test data from file
	testDataPath := filepath.Join("./testdata", filename)
	if _, err := os.Stat(testDataPath); err == nil {
		data, err := os.ReadFile(testDataPath)
		require.NoError(t, err)

		// Parse and create objects
		manifests := singleDocToManifests(data)
		for _, manifest := range manifests {
			if manifest == "" {
				continue
			}

			var obj metav1.PartialObjectMetadata
			err := yaml.Unmarshal([]byte(manifest), &obj)
			require.NoError(t, err)

			switch obj.Kind {
			case "ApplicationPackageVersion":
				var apv v1alpha1.ApplicationPackageVersion
				err := yaml.Unmarshal([]byte(manifest), &apv)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &apv))
			case "PackageRepository":
				var repo v1alpha1.PackageRepository
				err := yaml.Unmarshal([]byte(manifest), &repo)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &repo))
			}
		}
	}

	return ctr, kubeClient
}

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{
				Layers: []crv1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{}}, nil
		},
	}, nil)

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "non-existent-apv"},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{FilesContent: map[string]string{
					"package.yaml": `name: test-package
descriptions:
  en: Test package
  ru: Ru Test package
category: Test
stage: Preview
type: Application
version: "1.0.0"
`,
					"version.json":   `{"version": "1.0.0"}`,
					"changelog.yaml": "features:\n- Added new feature\nfixes:\n- Fixed a bug\n",
				}}}, nil
			},
		}, nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("successful-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("registry error reconcile with golden file", func() {
		// Override with invalid package.yaml
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(nil, fmt.Errorf("registry error"))

		suite.setupController("registry-error-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.Error(suite.T(), err)
	})

	suite.Run("metadata parsing error reconcile with golden file", func() {
		// Override with invalid package.yaml
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{FilesContent: map[string]string{
					"package.yaml": `invalid: yaml: content: [unclosed`,
					"version.json": `{"version": "1.0.0"}`,
				}}}, nil
			},
		}, nil)

		suite.setupController("metadata-parsing-error-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.Error(suite.T(), err)
	})

	suite.Run("non-draft resource skip", func() {
		suite.setupController("non-draft-resource.yaml")
		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("two errors reconcile with golden file", func() {
		// Override with invalid package.yaml
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(nil, fmt.Errorf("registry error"))

		suite.setupController("two-errors-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.Error(suite.T(), err)
	})

	suite.Run("err-to-success reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{FilesContent: map[string]string{
					"package.yaml": `name: test-package
descriptions:
  en: Test package
  ru: Ru Test package
category: Test
stage: Preview
type: Application
version: "1.0.0"
`,
					"version.json": `{"version": "1.0.0"}`,
				}}}, nil
			},
		}, nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("error-to-success.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("no bundle image in registry", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{FilesContent: map[string]string{
					"package.yaml": `name: test-package
descriptions:
  en: Test package
  ru: Ru Test package
category: Test
stage: Preview
type: Application
version: "1.0.0"
`,
					"version.json": `{"version": "1.0.0"}`,
				}}}, nil
			},
		}, nil)
		dc.CRClient.DigestMock.When(ctx, "v1.0.0").Then("", &transport.Error{StatusCode: http.StatusNotFound})

		suite.setupController("no-bundle-image-in-registry.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)

		apv = suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		require.Equal(suite.T(), "false", apv.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry])
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getApplicationPackageVersion(name string) *v1alpha1.ApplicationPackageVersion {
	var apv v1alpha1.ApplicationPackageVersion
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &apv)
	require.NoError(suite.T(), err)
	return &apv
}
