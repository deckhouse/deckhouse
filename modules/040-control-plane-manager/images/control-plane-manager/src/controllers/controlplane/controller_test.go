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

package controlplane

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")
	golden     bool
	scheme     = runtime.NewScheme()
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	ctx context.Context

	client     client.Client
	controller *Reconciler

	testDataFileName string
	time             metav1.Time
}

// TODO: Example test case
// func (suite *ControllerTestSuite) TestReconcileBasic() {
// 	suite.Run("When control plane configuration is created", func() {
// 		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
//
// 		_, err := suite.controller.Reconcile(
// 			suite.ctx,
// 			reconcile.Request{},
// 		)
//
// 		require.NoError(suite.T(), err)
// 	})
// }

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.T().Failed() {
		return
	}

	suite.T().Log("Dumping all resources:")
	for _, obj := range []client.ObjectList{
		&corev1.SecretList{},
		&controlplanev1alpha1.ControlPlaneConfigurationList{},
	} {
		err := suite.client.List(suite.ctx, obj)
		if err != nil {
			suite.T().Logf("failed to list %T: %v", obj, err)
			continue
		}

		data, err := yaml.Marshal(obj)
		if err != nil {
			suite.T().Logf("failed to marshal %T: %v", obj, err)
			continue
		}

		suite.T().Logf("---\n%s", data)
	}
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *ControllerTestSuite) setupController(objs []client.Object) {
	suite.client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	suite.controller = &Reconciler{
		client: suite.client,
	}
}

func (suite *ControllerTestSuite) fetchTestFileData(fileName string) []client.Object {
	suite.testDataFileName = fileName
	data, err := os.ReadFile(filepath.Join("testdata", "cases", fileName))
	require.NoError(suite.T(), err, "failed to read test file")

	return suite.parseManifests(string(data))
}

func (suite *ControllerTestSuite) parseManifests(data string) []client.Object {
	manifests := mDelimiter.Split(data, -1)
	var objs []client.Object

	for _, manifest := range manifests {
		manifest = strings.TrimSpace(manifest)
		if manifest == "" {
			continue
		}

		obj := &runtime.Unknown{}
		err := yaml.Unmarshal([]byte(manifest), obj)
		require.NoError(suite.T(), err, "failed to unmarshal manifest")

		switch obj.TypeMeta.Kind {
		case "Secret":
			secret := &corev1.Secret{}
			err = yaml.Unmarshal([]byte(manifest), secret)
			require.NoError(suite.T(), err)
			objs = append(objs, secret)
		case "ControlPlaneConfiguration":
			cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
			err = yaml.Unmarshal([]byte(manifest), cpc)
			require.NoError(suite.T(), err)
			objs = append(objs, cpc)
		default:
			suite.T().Logf("unknown kind: %s", obj.TypeMeta.Kind)
		}
	}

	return objs
}
