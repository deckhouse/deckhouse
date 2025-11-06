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
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
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

	ctr := &reconciler{
		client: kubeClient,
		logger: log.NewNop(),
		dc:     dependency.NewDependencyContainer(),
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
description:
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
		result, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), ctrl.Result{RequeueAfter: requeueTime}, result)
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
		result, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), ctrl.Result{RequeueAfter: requeueTime}, result)
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
		result, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), ctrl.Result{RequeueAfter: requeueTime}, result)
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
description:
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

		suite.setupController("error-to-success.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: apv.Name},
		})
		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getApplicationPackageVersion(name string) *v1alpha1.ApplicationPackageVersion {
	var apv v1alpha1.ApplicationPackageVersion
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &apv)
	require.NoError(suite.T(), err)
	return &apv
}

func TestConvertLicensingEditions(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]PackageEdition
		expected map[string]v1alpha1.PackageEdition
	}{
		{
			name: "multiple editions",
			input: map[string]PackageEdition{
				"ee": {Available: true},
				"ce": {Available: false},
				"fe": {Available: true},
			},
			expected: map[string]v1alpha1.PackageEdition{
				"ee": {Available: true},
				"ce": {Available: false},
				"fe": {Available: true},
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]PackageEdition{},
			expected: map[string]v1alpha1.PackageEdition{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLicensingEditions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
