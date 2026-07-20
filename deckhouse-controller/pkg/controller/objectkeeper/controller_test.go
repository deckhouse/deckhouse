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

package objectkeeper

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ObjectKeeperControllerTestSuite))
}

type ObjectKeeperControllerTestSuite struct {
	reconcilertest.Suite

	ctr *ObjectKeeperController
}

func (suite *ObjectKeeperControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{&v1alpha1.ObjectKeeper{}, &corev1.Pod{}},
		SnapshotKinds:      []schema.GroupVersionKind{v1alpha1.SchemeGroupVersion.WithKind("ObjectKeeper")},
		ObjectNormalizers:  []reconcilertest.ObjectNormalizer{normalizeObjectKeeper()},
		GoldenMode:         reconcilertest.WholeDocument,
		WithDynamic:        true,
	})
}

// setupController seeds the cluster from a fixture file and wires the controller
// under test to the freshly built fake clients.
func (suite *ObjectKeeperControllerTestSuite) setupController(filename string) {
	suite.Seed(filename)

	suite.ctr = &ObjectKeeperController{
		Client:     suite.Client(),
		logger:     log.NewNop(),
		dyn:        suite.Dynamic(),
		dc:         dependency.NewDependencyContainer(),
		restMapper: suite.RESTMapper(),
	}
}

func (suite *ObjectKeeperControllerTestSuite) TestCreateReconcile() {
	suite.Run("Check that ObjectKeeper immediately deleted", func() {
		suite.setupController("changedUID-with-ttl.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("changed-uid-with-ttl", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check pending phase with missingTTL condition", func() {
		suite.setupController("pending-missingTTL.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("pending-missing-ttl", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check tracking phase with followObject", func() {
		suite.setupController("followObject.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("follow-obj", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check that ObjectKeeper immediately deleted (ttl)", func() {
		suite.setupController("ttl-expired.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("ttl-expired", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check pending phase with MissingFollowObjectRef condition", func() {
		suite.setupController("pending-missingFollowObjectRef.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("missing-follow-objref", ""))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check that ObjectKeeper immediately deleted (missing FollowObject)", func() {
		suite.setupController("missingFollowObject.yaml")
		_, err := suite.ctr.Reconcile(context.TODO(), suite.Request("missing-follow-obj", ""))
		require.NoError(suite.T(), err)
	})
}

// normalizeObjectKeeper stabilises the timestamps and the (otherwise time-based)
// expiry message so golden snapshots stay deterministic.
func normalizeObjectKeeper() reconcilertest.ObjectNormalizer {
	constantTime := metav1.NewTime(time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC))

	return func(obj client.Object) {
		keeper, ok := obj.(*v1alpha1.ObjectKeeper)
		if !ok {
			return
		}

		shouldUpdateMessage := keeper.Status.Phase == v1alpha1.PhaseExpiring
		if keeper.Status.LostAt != nil {
			keeper.Status.LostAt = &constantTime
		}
		for i := range keeper.Status.Conditions {
			cond := &keeper.Status.Conditions[i]
			cond.LastTransitionTime = constantTime
			if shouldUpdateMessage {
				cond.Message = "TTL expires at 2099-01-01T20:00:00"
			}
		}
	}
}
