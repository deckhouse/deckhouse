// Copyright 2025 Flant JSC
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

package packagerepositoryoperation

import (
	"bytes"
	"context"
	"flag"
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

	var operationList v1alpha1.PackageRepositoryOperationList
	err := suite.kubeClient.List(context.TODO(), &operationList)
	require.NoError(suite.T(), err)

	for _, item := range operationList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var appList v1alpha1.ApplicationList
	err = suite.kubeClient.List(context.TODO(), &appList)
	require.NoError(suite.T(), err)

	for _, item := range appList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var apvList v1alpha1.ApplicationPackageVersionList
	err = suite.kubeClient.List(context.TODO(), &apvList)
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

	// Normalize timestamps for consistent golden files
	resultStr := result.String()
	resultStr = regexp.MustCompile(`startTime: "[^"]*"`).ReplaceAllString(resultStr, `startTime: "2025-10-31T12:00:00Z"`)
	resultStr = regexp.MustCompile(`syncTime: "[^"]*"`).ReplaceAllString(resultStr, `syncTime: "2025-10-31T12:00:00Z"`)
	resultStr = regexp.MustCompile(`completionTime: "[^"]*"`).ReplaceAllString(resultStr, `completionTime: "2025-10-31T12:00:00Z"`)

	return []byte(resultStr)
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
		WithStatusSubresource(&v1alpha1.PackageRepositoryOperation{}).
		WithStatusSubresource(&v1alpha1.ApplicationPackage{}).
		WithStatusSubresource(&v1alpha1.ApplicationPackageVersion{}).
		WithStatusSubresource(&v1alpha1.PackageRepository{}).
		Build()

	ctr := &reconciler{
		client: kubeClient,
		logger: log.NewNop(),
		dc:     dependency.NewMockedContainer(),
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
			case "PackageRepositoryOperation":
				var operation v1alpha1.PackageRepositoryOperation
				err := yaml.Unmarshal([]byte(manifest), &operation)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &operation))
			case "Application":
				var app v1alpha1.Application
				err := yaml.Unmarshal([]byte(manifest), &app)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &app))
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

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "non-existent-operation"},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("package repository not found", func() {
		suite.setupController("package-repository-not-found.yaml")
		operation := suite.getPackageRepositoryOperation("test-repo-scan-1571326380")

		err := repeat(20, func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("registry client creation failed", func() {
		dc := dependency.NewMockedContainer()
		// Set CRClient to nil to simulate GetRegistryClient failure
		dc.CRClient = nil

		suite.setupController("registry-client-failed.yaml", withDependencyContainer(dc))
		operation := suite.getPackageRepositoryOperation("test-repo-scan-1571326380")

		err := repeat(20, func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("package listing failed", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ListTagsMock.Return(nil, assert.AnError)

		suite.setupController("package-listing-failed.yaml", withDependencyContainer(dc))
		operation := suite.getPackageRepositoryOperation("test-repo-scan-1571326380")

		err := repeat(20, func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("successful package discovery", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
			ConfigFileStub: func() (*crv1.ConfigFile, error) {
				return &crv1.ConfigFile{
					Config: crv1.Config{
						Labels: map[string]string{
							"io.deckhouse.package.type": "Application",
						},
					},
				}, nil
			},
		}, nil)
		dc.CRClient.ListTagsMock.Return([]string{"test-package"}, nil)

		suite.setupController("successful-discovery.yaml", withDependencyContainer(dc))
		operation := suite.getPackageRepositoryOperation("test-repo-scan-1571326380")

		err := repeat(20, func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("successful completion", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{
			ManifestStub: func() (*crv1.Manifest, error) {
				return &crv1.Manifest{
					Layers: []crv1.Descriptor{},
				}, nil
			},
			LayersStub: func() ([]crv1.Layer, error) {
				return []crv1.Layer{&utils.FakeLayer{}}, nil
			},
			ConfigFileStub: func() (*crv1.ConfigFile, error) {
				return &crv1.ConfigFile{
					Config: crv1.Config{
						Labels: map[string]string{
							"io.deckhouse.package.type": "Application",
						},
					},
				}, nil
			},
		}, nil)
		dc.CRClient.ListTagsMock.Return([]string{"v1.0.0"}, nil)

		suite.setupController("successful-completion.yaml", withDependencyContainer(dc))
		operation := suite.getPackageRepositoryOperation("test-repo-scan-1571326380")

		err := repeat(20, func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getPackageRepositoryOperation(name string) *v1alpha1.PackageRepositoryOperation {
	var operation v1alpha1.PackageRepositoryOperation
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &operation)
	require.NoError(suite.T(), err)
	return &operation
}

func repeat(times int, fn func() error) error {
	for i := 0; i < times; i++ {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}
