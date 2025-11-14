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

package application

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

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
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	packagestatusservice "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status-package-service"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
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

	ctx := context.Background()
	err := suite.ctr.statusService.WaitForIdle(ctx)
	require.NoError(suite.T(), err)
	time.Sleep(3 * time.Second)

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

	var appList v1alpha1.ApplicationList
	err := suite.kubeClient.List(context.TODO(), &appList)
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
		WithStatusSubresource(&v1alpha1.Application{}).
		Build()

	pm := applicationpackage.NewStubPackageOperator(kubeClient, log.NewNop())
	eventChannel := make(chan packagestatusservice.PackageEvent, 100)
	pm.SetEventChannel(eventChannel)

	statusService := &StatusService{
		client:       kubeClient,
		logger:       log.NewNop(),
		pm:           pm,
		dc:           dependency.NewMockedContainer(),
		eventChannel: eventChannel,
	}

	go statusService.Start(context.Background())

	ctr := &reconciler{
		client:        kubeClient,
		logger:        log.NewNop(),
		pm:            pm,
		dc:            dependency.NewMockedContainer(),
		statusService: statusService,
		// exts:   extenders.NewExtendersStack(new(d8edition.Edition), nil, log.NewNop()),
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
			NamespacedName: types.NamespacedName{Name: "non-existent-app"},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: app.Name, Namespace: app.Namespace},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("version not found", func() {
		suite.setupController("version-not-found.yaml")
		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: app.Name, Namespace: app.Namespace},
		})
		require.NoError(suite.T(), err)
		app = suite.getApplication("test-app", "foobar")
		require.NotEmpty(suite.T(), app.Status.Conditions)
		require.Equal(suite.T(), v1alpha1.ApplicationConditionReasonVersionNotFound, app.Status.Conditions[0].Reason)
	})

	suite.Run("version is draft", func() {
		suite.setupController("version-is-draft.yaml")
		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: app.Name, Namespace: app.Namespace},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with some falses", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile-some-falses.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: app.Name, Namespace: app.Namespace},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with all falses", func() {
		requirements.RegisterCheck("k8s", func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
			v, _ := getter.Get("global.discovery.kubernetesVersion")
			if v != requirementValue {
				return false, errors.New("min k8s version failed")
			}

			return true, nil
		})
		requirements.SaveValue("global.discovery.kubernetesVersion", "1.19.0")

		dc := dependency.NewMockedContainer()

		suite.setupController("successful-reconcile-all-falses.yaml", withDependencyContainer(dc))

		app := suite.getApplication("test-app", "foobar")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: app.Name, Namespace: app.Namespace},
		})
		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getApplication(name string, namespace string) *v1alpha1.Application {
	var app v1alpha1.Application
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &app)
	require.NoError(suite.T(), err)
	return &app
}
