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
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/kube-client/manifest/releaseutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	ff "k8s.io/client-go/dynamic/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

var golden bool

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ObjectKeeperControllerTestSuite))
}

// Test suite structure
type ObjectKeeperControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *ObjectKeeperController

	testDataFileName string
}

func (suite *ObjectKeeperControllerTestSuite) TearDownSubTest() {
	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	got := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, got, 0666)
		require.NoError(suite.T(), err)
	} else {
		exp, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		assert.YAMLEq(suite.T(), string(exp), string(got))
	}
}

func (suite *ObjectKeeperControllerTestSuite) SetupSuite() {
	flag.Parse()
}

func (suite *ObjectKeeperControllerTestSuite) setupController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)
	var initObjects = make([]client.Object, 0, len(manifests))

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}
	
	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithRESTMapper(restMapper).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ObjectKeeper{}, &corev1.Pod{}).Build()
	dc := dependency.NewDependencyContainer()
	dynCl := ConvertToDynamic(cl, sc, initObjects...)

	rec := &ObjectKeeperController{
		Client:     cl,
		logger:     log.NewNop(),
		dyn:        dynCl,
		dc:         dc,
		restMapper: restMapper,
	}

	suite.ctr = rec
	suite.kubeClient = cl
}


func (suite *ObjectKeeperControllerTestSuite) TestCreateReconcile() {
	suite.Run("Check that ObjectKeeper immediately deleted", func() {
		suite.setupController(string(suite.fetchTestFileData("changedUID-with-ttl.yaml")))

		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("changed-uid-with-ttl"))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check pending phase with missingTTL condition", func() {
		suite.setupController(string(suite.fetchTestFileData("pending-missingTTL.yaml")))
		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("pending-missing-ttl"))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check tracking phase with followObject", func() {
		suite.setupController(string(suite.fetchTestFileData("followObject.yaml")))
		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("follow-obj"))
		require.NoError(suite.T(), err)
	})

	suite.Run("Check that ObjectKeeper immediately deleted (ttl)", func() {
		suite.setupController(string(suite.fetchTestFileData("ttl-expired.yaml")))
		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("ttl-expired"))
		require.NoError(suite.T(), err)
	})
	suite.Run("Check pending phase with MissingFollowObjectRef condition", func() {
		suite.setupController(string(suite.fetchTestFileData("pending-missingFollowObjectRef.yaml")))
		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("missing-follow-objref"))
		require.NoError(suite.T(), err)
	})
	suite.Run("Check that ObjectKeeper immediately deleted (missing FollowObject)", func() {
		suite.setupController(string(suite.fetchTestFileData("missingFollowObject.yaml")))
		_, err := suite.ctr.Reconcile(context.TODO(), suite.requestFor("missing-follow-obj"))
		require.NoError(suite.T(), err)
	})
}

func (suite *ObjectKeeperControllerTestSuite) assembleInitObject(obj string) client.Object {
	var res client.Object
	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(suite.T(), err)
	
	switch typ.Kind {
	case "ObjectKeeper":
		var ret v1alpha1.ObjectKeeper
		err = yaml.Unmarshal([]byte(obj), &ret)
		require.NoError(suite.T(), err)
		res = &ret
	case "Pod":
		var pod corev1.Pod
		err = yaml.Unmarshal([]byte(obj), &pod)
		require.NoError(suite.T(), err)
		res = &pod
	default:
		require.Fail(suite.T(), "unknown Kind:"+typ.Kind)
	}

	return res
}

func (suite *ObjectKeeperControllerTestSuite) fetchTestFileData(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}

func (suite *ObjectKeeperControllerTestSuite) fetchResults() []byte {

	result := bytes.NewBuffer(nil)
	constantTime := metav1.NewTime(time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC))

	var retList v1alpha1.ObjectKeeperList
	err := suite.kubeClient.List(context.TODO(), &retList)
	require.NoError(suite.T(), err)
	for _, item := range retList.Items {
		shouldUpdateMessage := item.Status.Phase == v1alpha1.PhaseExpiring
		if item.Status.LostAt != nil {
			item.Status.LostAt =  &constantTime
		}
		for i := range item.Status.Conditions {
			cond := &item.Status.Conditions[i]
			cond.LastTransitionTime = constantTime
			if shouldUpdateMessage {
				cond.Message = "TTL expires at 2099-01-01T20:00:00" // fix flaky test
			}
		}
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *ObjectKeeperControllerTestSuite) requestFor(name string) ctrl.Request {
	var ret v1alpha1.ObjectKeeper
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &ret)
	require.NoError(suite.T(), err)

	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}
func ConvertToDynamic(c client.Client, scheme *runtime.Scheme, objects ...client.Object) dynamic.Interface {
    var runtimeObjs []runtime.Object
    
    for _, obj := range objects {
        unstructuredObj := &unstructured.Unstructured{}
        if err := scheme.Convert(obj, unstructuredObj, nil); err == nil {
            runtimeObjs = append(runtimeObjs, unstructuredObj)
        }
    }
    
    return ff.NewSimpleDynamicClient(scheme, runtimeObjs...)
}
