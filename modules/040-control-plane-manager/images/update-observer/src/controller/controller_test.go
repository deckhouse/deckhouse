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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	helmreleaseutil "helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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
	suite.Run("When cluster is up to date", func() {
		suite.setupController(suite.fetchTestFileData("up-to-date.yaml"))

		_, err := suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{},
		)

		require.NoError(suite.T(), err)
	})
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
	var res client.Object
	metaType := new(runtime.TypeMeta)

	err := yaml.Unmarshal([]byte(strObj), &metaType)
	require.NoError(suite.T(), err)

	switch metaType.Kind {
	case "Secret":
		secret := new(corev1.Secret)
		err = yaml.Unmarshal([]byte(strObj), secret)
		require.NoError(suite.T(), err)
		res = secret
	case "ConfigMap":
		cm := new(corev1.ConfigMap)
		err = yaml.Unmarshal([]byte(strObj), cm)
		require.NoError(suite.T(), err)
		res = cm
	case "Node":
		node := new(corev1.Node)
		err = yaml.Unmarshal([]byte(strObj), node)
		require.NoError(suite.T(), err)
		res = node
	case "Pod":
		pod := new(corev1.Pod)
		err = yaml.Unmarshal([]byte(strObj), pod)
		require.NoError(suite.T(), err)
		res = pod
	default:
		suite.T().Fatalf("unsupported kind: %s", metaType.Kind)
	}

	return res
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
