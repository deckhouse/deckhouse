/*
Copyright 2026 Flant JSC

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

package controlplanenode

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")
	scheme     = runtime.NewScheme()
)

func init() {
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	ctx        context.Context
	client     client.Client
	controller *Reconciler
}

const testNodeName = "master-1"

func (suite *ControllerTestSuite) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *ControllerTestSuite) setupController(objs []client.Object) {
	suite.client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	suite.controller = &Reconciler{
		client: suite.client,
	}
}

func (suite *ControllerTestSuite) reconcile() {
	_, err := suite.controller.Reconcile(suite.ctx, reconcile.Request{
		NamespacedName: client.ObjectKey{Name: testNodeName},
	})
	require.NoError(suite.T(), err)
}

func (suite *ControllerTestSuite) getControlPlaneOperations() []controlplanev1alpha1.ControlPlaneOperation {
	list := &controlplanev1alpha1.ControlPlaneOperationList{}
	err := suite.client.List(suite.ctx, list, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey: testNodeName,
	})
	require.NoError(suite.T(), err)
	return list.Items
}

// TestInitialControlPlaneNode verifies that when ControlPlaneNode master-1 has no status.components,
// ControlPlaneOperation objects are created for all components with correctly populated spec.
func (suite *ControllerTestSuite) TestInitialControlPlaneNode() {
	suite.Run("when status.components is empty, create operations for all components", func() {
		suite.setupController(suite.fetchTestFileData("initial-control-plane-node.yaml"))
		suite.reconcile()

		operations := suite.getControlPlaneOperations()
		expected := suite.loadGoldenOperations("initial-control-plane-operations.yaml")

		require.Len(suite.T(), operations, len(expected),
			"number of created operations should match golden file")
		suite.compareOperations(operations, expected)
	})
}

// TestPartialControlPlaneNode verifies that when only two component checksums are outdated in status,
// ControlPlaneOperation objects are created only for those two components.
func (suite *ControllerTestSuite) TestPartialControlPlaneNode() {
	suite.Run("when only two checksums are outdated, create operations only for those components", func() {
		suite.setupController(suite.fetchTestFileData("partial-control-plane-node.yaml"))
		suite.reconcile()

		operations := suite.getControlPlaneOperations()
		expected := suite.loadGoldenOperations("partial-control-plane-operations.yaml")

		require.Len(suite.T(), operations, len(expected),
			"number of created operations should match golden file")
		suite.compareOperations(operations, expected)
	})
}

// TestHotReloadOutdatedControlPlaneNode verifies that when only HotReload checksum is outdated,
// ControlPlaneOperation is created for HotReload with desiredChecksum set to the hot-reload checksum.
func (suite *ControllerTestSuite) TestHotReloadOutdatedControlPlaneNode() {
	suite.Run("when only HotReload checksum is outdated, create operation with desiredChecksum", func() {
		suite.setupController(suite.fetchTestFileData("hotreload-outdated-control-plane-node.yaml"))
		suite.reconcile()

		operations := suite.getControlPlaneOperations()
		expected := suite.loadGoldenOperations("hotreload-outdated-control-plane-operations.yaml")

		require.Len(suite.T(), operations, len(expected),
			"number of created operations should match golden file")
		suite.compareOperations(operations, expected)
	})
}

// TestUpToDateControlPlaneNode verifies that when all spec checksums already match status checksums,
// no new ControlPlaneOperation objects are created.
func (suite *ControllerTestSuite) TestUpToDateControlPlaneNode() {
	suite.Run("when all checksums are up-to-date, no operations should be created", func() {
		suite.setupController(suite.fetchTestFileData("up-to-date-control-plane-node.yaml"))
		suite.reconcile()

		operations := suite.getControlPlaneOperations()
		require.Empty(suite.T(), operations,
			"no operations should be created when all checksums match")
	})
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.T().Failed() {
		return
	}

	suite.T().Log("Test failed, dumping resources:")
	for _, obj := range []client.ObjectList{
		&controlplanev1alpha1.ControlPlaneNodeList{},
		&controlplanev1alpha1.ControlPlaneOperationList{},
	} {
		err := suite.client.List(suite.ctx, obj)
		if err != nil {
			suite.T().Logf("Failed to list %T: %v", obj, err)
			continue
		}

		data, err := yaml.Marshal(obj)
		if err != nil {
			suite.T().Logf("Failed to marshal %T: %v", obj, err)
			continue
		}

		suite.T().Logf("---\n%s", data)
	}
}

func (suite *ControllerTestSuite) fetchTestFileData(fileName string) []client.Object {
	data, err := os.ReadFile(filepath.Join("testdata", "cases", fileName))
	require.NoError(suite.T(), err, "failed to read test file %s", fileName)
	return suite.parseManifests(string(data))
}

func (suite *ControllerTestSuite) loadGoldenOperations(fileName string) []controlplanev1alpha1.ControlPlaneOperation {
	data, err := os.ReadFile(filepath.Join("testdata", "golden", fileName))
	require.NoError(suite.T(), err, "failed to read golden file %s", fileName)

	parts := mDelimiter.Split(string(data), -1)
	var operations []controlplanev1alpha1.ControlPlaneOperation
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		op := controlplanev1alpha1.ControlPlaneOperation{}
		require.NoError(suite.T(), yaml.Unmarshal([]byte(part), &op),
			"failed to unmarshal operation %d in golden file %s", i, fileName)
		operations = append(operations, op)
	}
	return operations
}

func (suite *ControllerTestSuite) compareOperations(
	actual []controlplanev1alpha1.ControlPlaneOperation,
	expected []controlplanev1alpha1.ControlPlaneOperation,
) {
	sortByComponent := func(ops []controlplanev1alpha1.ControlPlaneOperation) {
		sort.Slice(ops, func(i, j int) bool {
			return ops[i].Spec.Component < ops[j].Spec.Component
		})
	}

	sortedActual := make([]controlplanev1alpha1.ControlPlaneOperation, len(actual))
	copy(sortedActual, actual)
	sortedExpected := make([]controlplanev1alpha1.ControlPlaneOperation, len(expected))
	copy(sortedExpected, expected)

	sortByComponent(sortedActual)
	sortByComponent(sortedExpected)

	for i := range sortedActual {
		require.Equal(suite.T(), sortedExpected[i].Spec, sortedActual[i].Spec,
			"spec of operation for component %s must match golden file", sortedActual[i].Spec.Component)
	}
}

func (suite *ControllerTestSuite) parseManifests(data string) []client.Object {
	manifests := mDelimiter.Split(data, -1)
	var objs []client.Object

	for i, manifest := range manifests {
		manifest = strings.TrimSpace(manifest)
		if manifest == "" {
			continue
		}

		metaType := &runtime.TypeMeta{}
		err := yaml.Unmarshal([]byte(manifest), metaType)
		require.NoError(suite.T(), err, "failed to unmarshal manifest %d", i)

		if metaType.Kind == "" {
			suite.T().Logf("manifest %d has empty kind, skipping", i)
			continue
		}

		switch metaType.Kind {
		case "ControlPlaneNode":
			cpn := &controlplanev1alpha1.ControlPlaneNode{}
			require.NoError(suite.T(), yaml.Unmarshal([]byte(manifest), cpn),
				"failed to unmarshal ControlPlaneNode")
			objs = append(objs, cpn)
		default:
			suite.T().Logf("unknown kind: %s, skipping", metaType.Kind)
		}
	}

	return objs
}
