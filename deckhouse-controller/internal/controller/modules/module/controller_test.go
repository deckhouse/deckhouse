// Copyright 2026 Flant JSC
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

package module

import (
	"context"
	"sync"
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

type ControllerTestSuite struct {
	reconcilertest.Suite

	r       *reconciler
	manager *stubManager

	compareGolden bool
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha2.Module{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha2.SchemeGroupVersion.WithKind("Module"),
		},
		GoldenMode: reconcilertest.PerDocument,
	})
}

func (suite *ControllerTestSuite) BeforeTest(suiteName, testName string) {
	if suiteName == "ControllerTestSuite" && testName == "TestCreateReconcile" {
		suite.compareGolden = true
	}
}

func (suite *ControllerTestSuite) AfterTest(_, _ string) {
	suite.compareGolden = false
}

// TearDownSubTest only asserts golden for golden-driven subtests; the rest assert
// behaviour (manager calls, finalizers) directly.
func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.compareGolden {
		return
	}
	suite.AssertGolden()
}

func (suite *ControllerTestSuite) setupTestController(filename string) {
	suite.Seed(filename)
	suite.buildReconciler()
}

// func (suite *ControllerTestSuite) setupTestControllerRaw(raw string) {
// 	suite.SeedRaw("", []byte(raw))
// 	suite.buildReconciler()
// }

func (suite *ControllerTestSuite) buildReconciler() {
	suite.manager = &stubManager{}

	rec := &reconciler{
		init:    new(sync.WaitGroup),
		client:  suite.Client(),
		manager: suite.manager,
		logger:  log.NewNop(),
	}

	// simulate initialization
	rec.init.Add(1)
	rec.init.Done()
	suite.r = rec
}

func (suite *ControllerTestSuite) module(name string) *v1alpha2.Module {
	module := new(v1alpha2.Module)
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, module)
	require.NoError(suite.T(), err)

	return module
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("apply settings and add finalizer", func() {
		suite.setupTestController("create-module.yaml")

		_, err := suite.r.Reconcile(context.TODO(), suite.Request("test-module", ""))
		require.NoError(suite.T(), err)

		// mapping is passed through verbatim
		require.Equal(suite.T(), 1, suite.manager.calls)
		assert.Equal(suite.T(), "test-module", suite.manager.name)
		assert.Equal(suite.T(), 3, suite.manager.settingsVersion)
		assert.Equal(suite.T(), addonutils.Values{"foo": "bar"}, suite.manager.settings)
		assert.Equal(suite.T(), "NoResourceReconciliation", suite.manager.maintenance)
		require.NotNil(suite.T(), suite.manager.enabled)
		assert.True(suite.T(), *suite.manager.enabled, "enabled *bool is forwarded as-is")

		// finalizer is put in place for a future delete
		module := suite.module("test-module")
		assert.True(suite.T(), controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered))
	})

	suite.Run("nil settings map to an empty map", func() {
		suite.setupTestController("no-settings.yaml")

		_, err := suite.r.Reconcile(context.TODO(), suite.Request("no-settings", ""))
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), 1, suite.manager.calls)
		assert.Equal(suite.T(), addonutils.Values{}, suite.manager.settings, "nil Settings maps to an empty, non-nil map")
		assert.Nil(suite.T(), suite.manager.enabled, "unset enabled stays nil")
	})

	suite.Run("module not found", func() {
		suite.setupTestController("not-found.yaml")

		res, err := suite.r.Reconcile(context.TODO(), suite.Request("absent", ""))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), ctrl.Result{}, res)
		assert.Zero(suite.T(), suite.manager.calls, "manager must not be called when the module is gone")
	})
}

func (suite *ControllerTestSuite) TestDeleteReconcile() {
	suite.Run("remove finalizer and release the module", func() {
		suite.setupTestController("delete-module.yaml")

		_, err := suite.r.Reconcile(context.TODO(), suite.Request("test-module", ""))
		require.NoError(suite.T(), err)

		assert.Zero(suite.T(), suite.manager.calls, "delete is a no-op for the settings manager")

		// finalizer gone -> object is garbage-collected
		err = suite.Client().Get(context.TODO(), client.ObjectKey{Name: "test-module"}, new(v1alpha2.Module))
		assert.True(suite.T(), apierrors.IsNotFound(err), "module must be released after finalizer removal")
	})

	suite.Run("skip system module", func() {
		suite.setupTestController("delete-system-module.yaml")

		_, err := suite.r.Reconcile(context.TODO(), suite.Request(moduleGlobal, ""))
		require.NoError(suite.T(), err)

		assert.Zero(suite.T(), suite.manager.calls)

		// system module is skipped: finalizer is kept, object still present
		module := suite.module(moduleGlobal)
		assert.True(suite.T(), controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered))
	})
}

// stubManager records the arguments of the last UpdateModulesSettings call.
type stubManager struct {
	calls           int
	name            string
	settingsVersion int
	settings        addonutils.Values
	maintenance     string
	enabled         *bool
}

func (s *stubManager) UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool) {
	s.calls++
	s.name = name
	s.settingsVersion = settingsVersion
	s.settings = settings
	s.maintenance = maintenance
	s.enabled = enabled
}
