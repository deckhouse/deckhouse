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

package packagerepository

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

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

func (suite *ControllerTestSuite) setupController(filename string) {
	suite.testDataFileName = filename
	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), filename)
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var repoList v1alpha1.PackageRepositoryList
	err := suite.kubeClient.List(context.TODO(), &repoList)
	require.NoError(suite.T(), err)

	for _, item := range repoList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var operationList v1alpha1.PackageRepositoryOperationList
	err = suite.kubeClient.List(context.TODO(), &operationList)
	require.NoError(suite.T(), err)

	for _, item := range operationList.Items {
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
		WithStatusSubresource(&v1alpha1.PackageRepository{}).
		WithStatusSubresource(&v1alpha1.PackageRepositoryOperation{}).
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
			case "PackageRepository":
				var repo v1alpha1.PackageRepository
				err := yaml.Unmarshal([]byte(manifest), &repo)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &repo))
			case "PackageRepositoryOperation":
				var operation v1alpha1.PackageRepositoryOperation
				err := yaml.Unmarshal([]byte(manifest), &operation)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &operation))
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
			NamespacedName: types.NamespacedName{Name: "non-existent-repo"},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		suite.setupController("successful-reconcile.yaml")
		repo := suite.getPackageRepository("test-repo")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: repo.Name},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("active operation exists", func() {
		suite.setupController("active-operation.yaml")
		repo := suite.getPackageRepository("test-repo")
		result, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: repo.Name},
		})
		require.NoError(suite.T(), err)
		// Should requeue when active operation exists
		require.True(suite.T(), result.RequeueAfter > 0)
	})

	suite.Run("delete repository", func() {
		suite.setupController("delete-repository.yaml")
		repo := suite.getPackageRepository("test-repo")

		// Test the delete method directly
		err := suite.ctr.delete(context.TODO(), repo)
		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getPackageRepository(name string) *v1alpha1.PackageRepository {
	var repo v1alpha1.PackageRepository
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &repo)
	require.NoError(suite.T(), err)
	return &repo
}
