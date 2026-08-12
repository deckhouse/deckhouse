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

package controller

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	helmreleaseutil "helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")
	golden     bool
	scheme     = runtime.NewScheme()
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	ctx context.Context

	client     client.Client
	controller *reconciler

	testDataFileName string
	time             metav1.Time
}

func (suite *ControllerTestSuite) TestConfigMapIsValid() {
	synctest.Test(suite.T(), func(*testing.T) {
		suite.T().Setenv("ALLOWED_KUBERNETES_VERSIONS", "1.32,1.33,1.34,1.35,1.36")
		suite.T().Setenv("AUTOMATIC_KUBERNETES_VERSION", "1.34")

		// No ConfigMap in this fixture on purpose: the bootstrap path, created from the environment.
		suite.Run("When cluster is up to date", func() {
			suite.setVersionEnv("1.35", "Automatic", "1.35")
			suite.setupController(suite.fetchTestFileData("init-up-to-date.yaml"))

			_, err := suite.controller.Reconcile(
				suite.ctx,
				reconcile.Request{},
			)

			require.NoError(suite.T(), err)
		})
		suite.Run("When control plane component was failed", func() {
			suite.setVersionEnv("1.35", "Automatic", "1.35")
			suite.setupController(suite.fetchTestFileData("pods-failed.yaml"))

			_, err := suite.controller.Reconcile(
				suite.ctx,
				reconcile.Request{},
			)

			require.NoError(suite.T(), err)
		})
		suite.Run("When supported and automatic versions are set via env", func() {
			suite.setVersionEnv("1.33", "Automatic", "1.34")
			suite.setupController(suite.fetchTestFileData("versions-with-env.yaml"))

			_, err := suite.controller.Reconcile(
				suite.ctx,
				reconcile.Request{},
			)

			require.NoError(suite.T(), err)
		})
		suite.Run("When upgrade is in progress across multiple versions", func() {
			suite.setVersionEnv("1.35", "Automatic", "1.32")
			suite.setupController(suite.fetchTestFileData("upgrade-three-hops.yaml"))

			_, err := suite.controller.Reconcile(
				suite.ctx,
				reconcile.Request{},
			)

			require.NoError(suite.T(), err)
		})
	})
}

// A test that omits this is testing a Pod that could not have started.
func (suite *ControllerTestSuite) setVersionEnv(desiredVersion, updateMode, maxUsedVersion string) {
	suite.T().Setenv("DESIRED_KUBERNETES_VERSION", desiredVersion)
	suite.T().Setenv("KUBERNETES_UPDATE_MODE", updateMode)
	suite.T().Setenv("MAX_USED_KUBERNETES_VERSION", maxUsedVersion)
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	if suite.testDataFileName == "" {
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

func (suite *ControllerTestSuite) setupController(yamlDoc string) {
	ctx := context.Background()

	manifests := helmreleaseutil.SplitManifests(yamlDoc)
	initObjects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjects...).
		Build()

	rec := &reconciler{
		client: k8sClient,
	}

	suite.controller = rec
	suite.client = k8sClient
	suite.ctx = ctx

	timeString := "2025-12-24T15:00:00+00:00"
	t1, _ := time.Parse(time.RFC3339, timeString)
	suite.time = metav1.NewTime(t1)
}

func (suite *ControllerTestSuite) assembleInitObject(strObj string) client.Object {
	return assembleInitObject(suite.T(), strObj)
}

func assembleInitObject(t *testing.T, strObj string) client.Object {
	t.Helper()

	var res client.Object
	metaType := new(runtime.TypeMeta)

	err := yaml.Unmarshal([]byte(strObj), &metaType)
	require.NoError(t, err)

	switch metaType.Kind {
	case "Secret":
		secret := new(corev1.Secret)
		err = yaml.Unmarshal([]byte(strObj), secret)
		require.NoError(t, err)
		res = secret
	case "ConfigMap":
		cm := new(corev1.ConfigMap)
		err = yaml.Unmarshal([]byte(strObj), cm)
		require.NoError(t, err)
		res = cm
	case "Node":
		node := new(corev1.Node)
		err = yaml.Unmarshal([]byte(strObj), node)
		require.NoError(t, err)
		res = node
	case "Pod":
		pod := new(corev1.Pod)
		err = yaml.Unmarshal([]byte(strObj), pod)
		require.NoError(t, err)
		res = pod
	default:
		t.Fatalf("unsupported kind: %s", metaType.Kind)
	}

	return res
}

// withConfigMap=false drops the case's own ConfigMap, exercising the bootstrap path.
func newTestReconciler(t *testing.T, filename string, withConfigMap bool) (*reconciler, client.Client) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("./testdata/cases", filename))
	require.NoError(t, err)

	manifests := helmreleaseutil.SplitManifests(string(data))
	objects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := assembleInitObject(t, manifest)
		if _, isConfigMap := obj.(*corev1.ConfigMap); isConfigMap && !withConfigMap {
			continue
		}
		objects = append(objects, obj)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	return &reconciler{client: k8sClient}, k8sClient
}

func readClusterConfigMap(t *testing.T, ctx context.Context, c client.Client) *corev1.ConfigMap {
	t.Helper()

	cm := new(corev1.ConfigMap)
	err := c.Get(ctx, client.ObjectKey{Name: "d8-cluster-kubernetes", Namespace: "kube-system"}, cm)
	require.NoError(t, err)

	return cm
}

// A guessed value would be indistinguishable from a declared one to every reader.
func TestReconcileWritesNothingOnBrokenEnvironment(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Setenv("ALLOWED_KUBERNETES_VERSIONS", "1.32,1.33,1.34,1.35,1.36")
		t.Setenv("AUTOMATIC_KUBERNETES_VERSION", "1.34")
		t.Setenv("DESIRED_KUBERNETES_VERSION", "")
		t.Setenv("KUBERNETES_UPDATE_MODE", "Automatic")
		t.Setenv("MAX_USED_KUBERNETES_VERSION", "1.35")

		rec, k8sClient := newTestReconciler(t, "pods-failed.yaml", true)
		before := readClusterConfigMap(t, context.Background(), k8sClient)

		result, err := rec.Reconcile(context.Background(), reconcile.Request{})
		require.NoError(t, err)
		assert.Equal(t, requeueInterval, result.RequeueAfter, "a broken environment must requeue")

		after := readClusterConfigMap(t, context.Background(), k8sClient)
		assert.Equal(t, before.Data, after.Data)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion, "nothing may be written at all")
	})
}

// The controller writes data.spec and watches it, so its own write produces one more event.
func TestReconcileSelfTriggerConverges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Setenv("ALLOWED_KUBERNETES_VERSIONS", "1.32,1.33,1.34,1.35,1.36")
		t.Setenv("AUTOMATIC_KUBERNETES_VERSION", "1.34")
		t.Setenv("DESIRED_KUBERNETES_VERSION", "1.35")
		t.Setenv("KUBERNETES_UPDATE_MODE", "Automatic")
		t.Setenv("MAX_USED_KUBERNETES_VERSION", "1.35")

		ctx := context.Background()
		rec, k8sClient := newTestReconciler(t, "init-up-to-date.yaml", false)

		_, err := rec.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
		first := readClusterConfigMap(t, ctx, k8sClient)

		_, err = rec.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
		second := readClusterConfigMap(t, ctx, k8sClient)

		require.Equal(t, first.Data["spec"], second.Data["spec"])
		assert.False(t, getConfigMapSpecPredicate().Update(event.UpdateEvent{ObjectOld: first, ObjectNew: second}),
			"the second write must not enqueue a third reconcile")
	})
}

// The max-k8s-version label only moves once the cluster reaches UpToDate.
func TestAvailableVersionsFollowTheComputedMaxUsed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Setenv("ALLOWED_KUBERNETES_VERSIONS", "1.32,1.33,1.34,1.35,1.36")
		t.Setenv("AUTOMATIC_KUBERNETES_VERSION", "1.34")
		t.Setenv("DESIRED_KUBERNETES_VERSION", "1.35")
		t.Setenv("KUBERNETES_UPDATE_MODE", "Automatic")
		t.Setenv("MAX_USED_KUBERNETES_VERSION", "1.35")

		ctx := context.Background()
		// The fixture's label still says 1.32 — the cluster has not finished the hop yet.
		rec, k8sClient := newTestReconciler(t, "upgrade-three-hops.yaml", true)

		_, err := rec.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		cm := readClusterConfigMap(t, ctx, k8sClient)
		assert.Equal(t, "1.32", cm.Labels["max-k8s-version"])
		assert.Contains(t, cm.Data["status"], "availableVersions:\n- \"1.34\"\n- \"1.35\"\n- \"1.36\"\n")
	})
}

func (suite *ControllerTestSuite) fetchTestFileData(filename string) string {
	dir := "./testdata/cases"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return string(data)
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	cms := new(corev1.ConfigMapList)
	require.NoError(suite.T(), suite.client.List(suite.ctx, cms))

	for _, cm := range cms.Items {
		cm := cm.DeepCopy()

		cm.ResourceVersion = ""
		cm.UID = ""
		cm.ManagedFields = nil

		got, _ := yaml.Marshal(cm)
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
			s := strings.TrimSpace(split[i])
			if s != "" {
				result = append(result, s)
			}
		}
	}

	return result
}
