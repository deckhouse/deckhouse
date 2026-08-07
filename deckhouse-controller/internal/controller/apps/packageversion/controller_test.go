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
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr *reconciler
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{&v1alpha1.ApplicationPackageVersion{}},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ApplicationPackageVersion"),
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepository"),
		},
		GoldenMode:    reconcilertest.PerDocument,
		SeedViaCreate: true,
	})
}

func (suite *ControllerTestSuite) SetupSubTest() {
	reconcilertest.RespondHTTPOK()
}

type reconcilerOption func(*reconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dc = dc
		r.registry = registry.NewService(dc, log.NewNop())
	}
}

func (suite *ControllerTestSuite) setupController(filename string, options ...reconcilerOption) {
	suite.Seed(filename)

	dc := dependency.NewDependencyContainer()
	suite.ctr = &reconciler{
		client:   suite.Client(),
		logger:   log.NewNop(),
		dc:       dc,
		registry: registry.NewService(dc, log.NewNop()),
	}

	for _, opt := range options {
		opt(suite.ctr)
	}
}

const testPackageYAML = `name: test-package
descriptions:
  en: Test package
  ru: Ru Test package
disable:
  confirmation: true
  messages:
    ru: "RU disable message"
    en: "EN disable message"
category: Test
stage: Preview
type: Application
version: "1.0.0"
`

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	dependency.TestDC.CRClient.ImageMock.Return(reconcilertest.Image(nil), nil)

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, suite.Request("non-existent-apv", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"package.yaml":   testPackageYAML,
			"version.json":   `{"version": "1.0.0"}`,
			"changelog.yaml": "features:\n- Added new feature\nfixes:\n- Fixed a bug\n",
		}), nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("successful-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("registry error reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(nil, fmt.Errorf("registry error"))

		suite.setupController("registry-error-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.Error(suite.T(), err)
	})

	suite.Run("metadata parsing error reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"package.yaml": `invalid: yaml: content: [unclosed`,
			"version.json": `{"version": "1.0.0"}`,
		}), nil)

		suite.setupController("metadata-parsing-error-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.Error(suite.T(), err)
	})

	suite.Run("non-draft resource skip", func() {
		suite.setupController("non-draft-resource.yaml")
		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("two errors reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(nil, fmt.Errorf("registry error"))

		suite.setupController("two-errors-reconcile.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.Error(suite.T(), err)
	})

	suite.Run("err-to-success reconcile with golden file", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"package.yaml": testPackageYAML,
			"version.json": `{"version": "1.0.0"}`,
		}), nil)
		dc.CRClient.DigestMock.Return("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil)

		suite.setupController("error-to-success.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("no bundle image in registry", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(reconcilertest.Image(map[string]string{
			"package.yaml": testPackageYAML,
			"version.json": `{"version": "1.0.0"}`,
		}), nil)
		dc.CRClient.DigestMock.When(ctx, "v1.0.0").Then("", &transport.Error{StatusCode: http.StatusNotFound})

		suite.setupController("no-bundle-image-in-registry.yaml", withDependencyContainer(dc))

		apv := suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(apv.Name, ""))
		require.NoError(suite.T(), err)

		apv = suite.getApplicationPackageVersion("deckhouse-test-v1.0.0")
		require.Equal(suite.T(), "false", apv.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry])
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getApplicationPackageVersion(name string) *v1alpha1.ApplicationPackageVersion {
	var apv v1alpha1.ApplicationPackageVersion
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, &apv)
	require.NoError(suite.T(), err)
	return &apv
}
