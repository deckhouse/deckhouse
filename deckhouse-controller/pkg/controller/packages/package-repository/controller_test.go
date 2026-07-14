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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		StatusSubresources: []client.Object{
			&v1alpha1.PackageRepository{},
			&v1alpha1.PackageRepositoryOperation{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepository"),
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepositoryOperation"),
		},
		GoldenMode:    reconcilertest.PerDocument,
		SeedViaCreate: true,
	})
}

func (suite *ControllerTestSuite) setupController(filename string) {
	suite.Seed(filename)

	suite.ctr = &reconciler{
		client: suite.Client(),
		logger: log.NewNop(),
		dc:     dependency.NewMockedContainer(),
	}
}

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, suite.Request("non-existent-repo", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("successful reconcile with golden file", func() {
		suite.setupController("successful-reconcile.yaml")
		repo := suite.getPackageRepository("deckhouse")
		_, err := suite.ctr.Reconcile(ctx, suite.Request(repo.Name, ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("active operation exists", func() {
		suite.setupController("active-operation.yaml")
		repo := suite.getPackageRepository("deckhouse")
		result, err := suite.ctr.Reconcile(ctx, suite.Request(repo.Name, ""))
		require.NoError(suite.T(), err)
		// Should requeue when active operation exists
		require.True(suite.T(), result.RequeueAfter > 0)
	})

	suite.Run("delete repository", func() {
		suite.setupController("delete-repository.yaml")
		repo := suite.getPackageRepository("deckhouse")

		// Test the delete method directly
		err := suite.ctr.delete(context.TODO(), repo)
		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getPackageRepository(name string) *v1alpha1.PackageRepository {
	var repo v1alpha1.PackageRepository
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, &repo)
	require.NoError(suite.T(), err)
	return &repo
}
