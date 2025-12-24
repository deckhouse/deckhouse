/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	helmreleaseutil "helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"safe-updater/internal/constant"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")

	golden bool
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	klog.InitFlags(nil)
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	ctx context.Context

	client            client.Client
	controller        *reconciler
	testDataFileName  string
	testDaemonSetName string
	time              metav1.Time
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	if suite.testDataFileName == "" {
		return
	}

	goldenFile := filepath.Join("./testdata/cases", "golden", suite.testDataFileName)
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

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("daemonset-is-up-to-date", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-is-up-to-date.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pod-and-no-cilium-pod", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pod-and-no-cilium-pod.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pod-and-two-cilium-pods", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pod-and-two-cilium-pods.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pod-and-cilium-pod-is-not-up-to-date", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pod-and-cilium-pod-is-not-up-to-date.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pod-and-cilium-is-ready", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pod-and-cilium-is-ready.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pod-and-cilium-pod-not-ready", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pod-and-cilium-pod-not-ready.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pods-and-one-cilium-pod-is-ready", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pods-and-one-cilium-pod-is-ready.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-one-pod-is-not-ready-and-cilium-is-ready", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-one-pod-is-not-ready-and-cilium-is-ready.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pods-and-one-cilium-pod-is-up-to-date", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pods-and-one-cilium-pod-is-up-to-date.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})

	suite.Run("daemonset-updated-but-not-pods-and-one-pod-is-not-running-and-stale", func() {
		err := suite.setupController(suite.fetchTestFileData("daemonset-updated-but-not-pods-and-one-pod-is-not-running-and-stale.yaml"))
		require.NoError(suite.T(), err)

		ds := suite.getNodeLocalDNSDaemonSet()
		_, err = suite.controller.reconcileDaemonSet(suite.ctx, ds)
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) setupController(yamlDoc string) error {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	ctx := context.Background()

	manifests := helmreleaseutil.SplitManifests(yamlDoc)
	initObjects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

	rec := &reconciler{
		client: k8sClient,
	}

	suite.controller = rec
	suite.client = k8sClient
	suite.testDaemonSetName = constant.NodeLocalDNSDaemonSet
	suite.ctx = ctx

	timeString := "2025-11-30T22:08:41+00:00"
	t1, _ := time.Parse(time.RFC3339, timeString)
	suite.time = metav1.NewTime(t1)

	return nil
}

func (suite *ControllerTestSuite) assembleInitObject(strObj string) client.Object {
	raw := []byte(strObj)

	metaType := new(runtime.TypeMeta)
	err := yaml.Unmarshal(raw, metaType)
	require.NoError(suite.T(), err)

	var obj client.Object

	switch metaType.Kind {
	case "ControllerRevision":
		cr := new(appsv1.ControllerRevision)
		err = yaml.Unmarshal(raw, cr)
		require.NoError(suite.T(), err)
		obj = cr
	case "DaemonSet":
		ds := new(appsv1.DaemonSet)
		err = yaml.Unmarshal(raw, ds)
		require.NoError(suite.T(), err)
		obj = ds

	case "Pod":
		pod := new(corev1.Pod)
		err = yaml.Unmarshal(raw, pod)
		require.NoError(suite.T(), err)
		obj = pod
	}

	return obj
}

func (suite *ControllerTestSuite) fetchTestFileData(filename string) string {
	dir := "./testdata/cases"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return string(data)
}

func (suite *ControllerTestSuite) getNodeLocalDNSDaemonSet() *appsv1.DaemonSet {
	ds := new(appsv1.DaemonSet)
	err := suite.client.Get(suite.ctx, client.ObjectKey{Name: constant.NodeLocalDNSDaemonSet, Namespace: constant.NodeLocalDNSNamespace}, ds)
	require.NoError(suite.T(), err)

	return ds
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	pods := new(corev1.PodList)
	require.NoError(suite.T(), suite.client.List(suite.ctx, pods))

	for _, pod := range pods.Items {
		if !pod.DeletionTimestamp.IsZero() {
			pod.SetDeletionTimestamp(&suite.time)
		}
		got, _ := yaml.Marshal(pod)
		result.WriteString("---\n")
		result.Write(got)
	}

	dss := new(appsv1.DaemonSetList)
	require.NoError(suite.T(), suite.client.List(suite.ctx, dss))

	for _, ds := range dss.Items {
		got, _ := yaml.Marshal(ds)
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
