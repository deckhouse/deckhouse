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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func (suite *ControllerTestSuite) TestReconcileCreateControlPlaneConfiguration() {
	suite.Run("When secrets exist, ControlPlaneConfiguration should be created with correct checksums", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		_, err := suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: "control-plane",
				},
			},
		)

		require.NoError(suite.T(), err)

		// ControlPlaneConfiguration must be created/updated
		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: "control-plane"}, cpc)
		require.NoError(suite.T(), err, "ControlPlaneConfiguration should exist")

		// Spec must be populated
		require.NotNil(suite.T(), cpc.Spec.Components, "Components should not be empty")
		require.NotEmpty(suite.T(), cpc.Spec.PKIChecksum, "PKIChecksum should not be empty")

		// Component checksums must exist
		require.NotNil(suite.T(), cpc.Spec.Components.Etcd, "Etcd component should not be empty")
		require.NotEmpty(suite.T(), cpc.Spec.Components.Etcd.Checksum, "Etcd checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeAPIServer, "KubeAPIServer component should not be empty")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeAPIServer.Checksum, "KubeAPIServer checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeControllerManager, "KubeControllerManager component should not be empty")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeControllerManager.Checksum, "KubeControllerManager checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeScheduler, "KubeScheduler component should not be empty")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeScheduler.Checksum, "KubeScheduler checksum should not be empty")

		suite.T().Logf("PKI Checksum: %s", cpc.Spec.PKIChecksum)
		suite.T().Logf("Etcd Checksum: %s", cpc.Spec.Components.Etcd.Checksum)
		suite.T().Logf("KubeAPIServer Checksum: %s", cpc.Spec.Components.KubeAPIServer.Checksum)
		suite.T().Logf("KubeControllerManager Checksum: %s", cpc.Spec.Components.KubeControllerManager.Checksum)
		suite.T().Logf("KubeScheduler Checksum: %s", cpc.Spec.Components.KubeScheduler.Checksum)
	})
}

func (suite *ControllerTestSuite) TestReconcileUpdateControlPlaneConfiguration() {
	suite.Run("When secrets are updated, ControlPlaneConfiguration checksums should change", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		// First reconcile
		_, err := suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: "control-plane",
				},
			},
		)
		require.NoError(suite.T(), err)

		// Get initial checksums
		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: "control-plane"}, cpc)
		require.NoError(suite.T(), err)

		oldPKIChecksum := cpc.Spec.PKIChecksum
		oldEtcdChecksum := cpc.Spec.Components.Etcd.Checksum

		// Update PKI secret
		pkiSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      "d8-pki",
			Namespace: "kube-system",
		}, pkiSecret)
		require.NoError(suite.T(), err)

		// Modify PKI secret data
		pkiSecret.Data["ca.crt"] = []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5FVyBDQSBDRVJUCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=")
		err = suite.client.Update(suite.ctx, pkiSecret)
		require.NoError(suite.T(), err)

		// Second reconcile
		_, err = suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: "control-plane",
				},
			},
		)
		require.NoError(suite.T(), err)

		// Get updated checksums
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: "control-plane"}, cpc)
		require.NoError(suite.T(), err)

		// PKI checksum should have changed
		require.NotEqual(suite.T(), oldPKIChecksum, cpc.Spec.PKIChecksum, "PKI checksum should change after secret update")

		// Etcd checksum should remain the same (we didn't update its secret data)
		require.Equal(suite.T(), oldEtcdChecksum, cpc.Spec.Components.Etcd.Checksum, "Etcd checksum should remain unchanged")

		suite.T().Logf("Old PKI Checksum: %s", oldPKIChecksum)
		suite.T().Logf("New PKI Checksum: %s", cpc.Spec.PKIChecksum)
	})
}

func (suite *ControllerTestSuite) TestReconcileComponentChecksumChange() {
	suite.Run("When component manifest is updated, only its checksum should change", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		// First reconcile
		_, err := suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: "control-plane",
				},
			},
		)
		require.NoError(suite.T(), err)

		// Get initial checksums
		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: "control-plane"}, cpc)
		require.NoError(suite.T(), err)

		oldEtcdChecksum := cpc.Spec.Components.Etcd.Checksum
		oldAPIServerChecksum := cpc.Spec.Components.KubeAPIServer.Checksum

		// Update etcd manifest in config secret
		configSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      "d8-control-plane-manager-config",
			Namespace: "kube-system",
		}, configSecret)
		require.NoError(suite.T(), err)

		// Modify etcd.yaml
		configSecret.Data["etcd.yaml"] = []byte("YXBpVmVyc2lvbjogdjIKa2luZDogUG9kCm1ldGFkYXRhOgogIG5hbWU6IGV0Y2QtdXBkYXRlZA==")
		err = suite.client.Update(suite.ctx, configSecret)
		require.NoError(suite.T(), err)

		// Second reconcile
		_, err = suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: "control-plane",
				},
			},
		)
		require.NoError(suite.T(), err)

		// Get updated checksums
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: "control-plane"}, cpc)
		require.NoError(suite.T(), err)

		// Etcd checksum should have changed
		require.NotEqual(suite.T(), oldEtcdChecksum, cpc.Spec.Components.Etcd.Checksum, "Etcd checksum should change after manifest update")

		// API Server checksum should remain the same
		require.Equal(suite.T(), oldAPIServerChecksum, cpc.Spec.Components.KubeAPIServer.Checksum, "KubeAPIServer checksum should remain unchanged")

		suite.T().Logf("Old Etcd Checksum: %s", oldEtcdChecksum)
		suite.T().Logf("New Etcd Checksum: %s", cpc.Spec.Components.Etcd.Checksum)
	})
}

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
		case "Secret":
			secret := &corev1.Secret{}
			err = yaml.Unmarshal([]byte(manifest), secret)
			require.NoError(suite.T(), err, "failed to unmarshal Secret")
			objs = append(objs, secret)
		case "ControlPlaneConfiguration":
			cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
			err = yaml.Unmarshal([]byte(manifest), cpc)
			require.NoError(suite.T(), err, "failed to unmarshal ControlPlaneConfiguration")
			objs = append(objs, cpc)
		default:
			suite.T().Logf("unknown kind: %s", metaType.Kind)
		}
	}

	return objs
}
