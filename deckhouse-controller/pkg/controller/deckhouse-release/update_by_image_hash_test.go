/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

type storageMockCounterAddParams struct {
	metric string
	value  float64
	labels map[string]string
}

func (suite *ControllerTestSuite) TestUpdateByImageHash() {
	ctx := context.Background()

	suite.Run("No new deckhouse image", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1", nil
		})

		suite.setupController("dev-no-new-deckhouse-image.yaml", initValues, embeddedMUP, withDependencyContainer(dc))
		suite.metricStorage.CounterAddMock.Times(3).Set(counterAddMock(suite.T()))

		leaderPod, err := suite.ctr.getDeckhouseLatestPod(ctx)
		require.NoError(suite.T(), err)

		err = suite.ctr.tagUpdate(ctx, leaderPod)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have new deckhouse image", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.DigestMock.Set(func(_ string) (s1 string, err error) {
			return "sha256:123456", nil
		})

		ds := &helpers.DeckhouseSettings{}
		ds.Update.Mode = embeddedMUP.Update.Mode
		ds.Update.Windows = embeddedMUP.Update.Windows
		ds.Update.DisruptionApprovalMode = "Auto"

		suite.setupControllerSettings("dev-have-new-deckhouse-image.yaml", initValues, ds, withDependencyContainer(dc))
		suite.metricStorage.CounterAddMock.Times(3).Set(counterAddMock(suite.T()))

		leaderPod, err := suite.ctr.getDeckhouseLatestPod(ctx)
		require.NoError(suite.T(), err)

		err = suite.ctr.tagUpdate(ctx, leaderPod)
		require.NoError(suite.T(), err)
	})
}

// TODO: use somthing like this:
// suite.metricStorage.CounterAddMock.Expect("deckhouse_registry_check_total", 1, map[string]string{})
// suite.metricStorage.CounterAddMock.Expect("deckhouse_kube_image_digest_check_total", 1, map[string]string{})
// suite.metricStorage.CounterAddMock.Expect("deckhouse_kube_image_digest_check_success", 1, map[string]string{})
func counterAddMock(t *testing.T) func(string, float64, map[string]string) {
	t.Helper()

	return func(metric string, value float64, labels map[string]string) {
		params := storageMockCounterAddParams{
			metric: metric,
			value:  value,
			labels: labels,
		}

		if reflect.DeepEqual(params, storageMockCounterAddParams{"deckhouse_registry_check_total", 1, map[string]string{}}) ||
			reflect.DeepEqual(params, storageMockCounterAddParams{"deckhouse_kube_image_digest_check_total", 1, map[string]string{}}) ||
			reflect.DeepEqual(params, storageMockCounterAddParams{"deckhouse_kube_image_digest_check_success", 1, map[string]string{}}) {
			return
		}

		t.Fatalf("Unexpected arguments: %+v\n", params)
	}
}
