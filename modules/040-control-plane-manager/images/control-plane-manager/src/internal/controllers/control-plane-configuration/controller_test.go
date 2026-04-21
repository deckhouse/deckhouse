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

package controlplaneconfiguration

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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
}

const testNodeName = "master-1"

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

func (suite *ControllerTestSuite) reconcile() {
	_, err := suite.controller.Reconcile(suite.ctx, reconcile.Request{
		NamespacedName: client.ObjectKey{Name: testNodeName},
	})
	require.NoError(suite.T(), err)
}

func (suite *ControllerTestSuite) getControlPlaneNode() *controlplanev1alpha1.ControlPlaneNode {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	err := suite.client.Get(suite.ctx, client.ObjectKey{Name: testNodeName}, cpn)
	require.NoError(suite.T(), err, "ControlPlaneNode should exist")
	return cpn
}

func (suite *ControllerTestSuite) getPKISecret() *corev1.Secret {
	s := &corev1.Secret{}
	err := suite.client.Get(suite.ctx, client.ObjectKey{
		Name:      constants.PkiSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, s)
	require.NoError(suite.T(), err)
	return s
}

func (suite *ControllerTestSuite) getConfigSecret() *corev1.Secret {
	s := &corev1.Secret{}
	err := suite.client.Get(suite.ctx, client.ObjectKey{
		Name:      constants.ControlPlaneManagerConfigSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, s)
	require.NoError(suite.T(), err)
	return s
}

// TestReconcileCreatesControlPlaneNode verifies that reconciling a master Node creates a ControlPlaneNode with all checksum fields populated
func (suite *ControllerTestSuite) TestReconcileCreatesControlPlaneNode() {
	suite.Run("ControlPlaneNode should be created with all checksums non-empty", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()

		require.NotEmpty(suite.T(), cpn.Spec.CAChecksum, "CAChecksum should not be empty")
		require.NotEmpty(suite.T(), cpn.Spec.HotReloadChecksum, "HotReloadChecksum should not be empty")
		require.Equal(suite.T(), constants.HeritageLabelValue, cpn.Labels[constants.HeritageLabelKey], "ControlPlaneNode should have heritage label")

		require.NotNil(suite.T(), cpn.Spec.Components.Etcd, "Etcd should not be nil")
		require.NotEmpty(suite.T(), cpn.Spec.Components.Etcd.Checksums.Config, "Etcd checksum should not be empty")
		require.NotEmpty(suite.T(), cpn.Spec.Components.Etcd.Checksums.PKI, "Etcd pki checksum should not be empty")

		require.NotNil(suite.T(), cpn.Spec.Components.KubeAPIServer, "KubeAPIServer should not be nil")
		require.NotEmpty(suite.T(), cpn.Spec.Components.KubeAPIServer.Checksums.Config, "KubeAPIServer checksum should not be empty")
		require.NotEmpty(suite.T(), cpn.Spec.Components.KubeAPIServer.Checksums.PKI, "KubeAPIServer pki checksum should not be empty")

		require.NotNil(suite.T(), cpn.Spec.Components.KubeControllerManager, "KubeControllerManager should not be nil")
		require.NotEmpty(suite.T(), cpn.Spec.Components.KubeControllerManager.Checksums.Config, "KubeControllerManager checksum should not be empty")
		require.Empty(suite.T(), cpn.Spec.Components.KubeControllerManager.Checksums.PKI, "KubeControllerManager must have no PKIChecksum (no leaf certs)")

		require.NotNil(suite.T(), cpn.Spec.Components.KubeScheduler, "KubeScheduler should not be nil")
		require.NotEmpty(suite.T(), cpn.Spec.Components.KubeScheduler.Checksums.Config, "KubeScheduler checksum should not be empty")
		require.Empty(suite.T(), cpn.Spec.Components.KubeScheduler.Checksums.PKI, "KubeScheduler must have no PKIChecksum (no leaf certs)")
	})
}

// TestGoldenControlPlaneNode compares the reconciled ControlPlaneNode spec with a pre-computed golden file stored in testdata/golden/
// This catches regressions in checksum logic: if CalculateComponentChecksum changes, the hardcoded golden values will no longer match
// To regenerate the golden file after intentional changes, run: UPDATE_GOLDEN=true go test ./... or make test-golden-update
func (suite *ControllerTestSuite) TestGoldenControlPlaneNode() {
	suite.Run("ControlPlaneNode spec should match golden file", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()

		goldenPath := filepath.Join("testdata", "golden", "basic-config.yaml")

		if os.Getenv("UPDATE_GOLDEN") == "true" {
			golden := &controlplanev1alpha1.ControlPlaneNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: controlplanev1alpha1.GroupVersion.String(),
					Kind:       "ControlPlaneNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: cpn.Name,
				},
				Spec: cpn.Spec,
			}
			data, err := yaml.Marshal(golden)
			require.NoError(suite.T(), err)
			require.NoError(suite.T(), os.MkdirAll(filepath.Dir(goldenPath), 0o755))
			require.NoError(suite.T(), os.WriteFile(goldenPath, data, 0o644))
			suite.T().Logf("Updated golden file: %s", goldenPath)
			return
		}

		goldenData, err := os.ReadFile(goldenPath)
		require.NoError(suite.T(), err,
			"golden file not found at %s; run with UPDATE_GOLDEN=true to generate it", goldenPath)

		var golden controlplanev1alpha1.ControlPlaneNode
		require.NoError(suite.T(), yaml.Unmarshal(goldenData, &golden))

		require.Equal(suite.T(), golden.Name, cpn.Name,
			"ControlPlaneNode name must match golden file")
		require.Equal(suite.T(), golden.Spec, cpn.Spec,
			"ControlPlaneNode spec must match golden file; if this is purposely, run with UPDATE_GOLDEN=true")
	})
}

// TestCAChecksumChangesOnPKISecretUpdate verifies that CAChecksum changes when d8-pki changes,
// while per-component PKIChecksum (cert-sans/encryption-algorithm) and ConfigChecksums remain stable.
func (suite *ControllerTestSuite) TestCAChecksumChangesOnPKISecretUpdate() {
	suite.Run("CAChecksum should change when PKI secret is updated; per-component PKIChecksum and ConfigChecksums should not", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()
		oldCAChecksum := cpn.Spec.CAChecksum
		oldEtcdConfigChecksum := cpn.Spec.Components.Etcd.Checksums.Config
		oldEtcdPKIChecksum := cpn.Spec.Components.Etcd.Checksums.PKI
		oldAPIServerPKIChecksum := cpn.Spec.Components.KubeAPIServer.Checksums.PKI

		pkiSecret := suite.getPKISecret()
		pkiSecret.Data["ca.crt"] = []byte("NEW-CA-CERT-CONTENT")
		require.NoError(suite.T(), suite.client.Update(suite.ctx, pkiSecret))

		suite.reconcile()

		cpn = suite.getControlPlaneNode()
		require.NotEqual(suite.T(), oldCAChecksum, cpn.Spec.CAChecksum,
			"CAChecksum should change after d8-pki secret update")
		require.Equal(suite.T(), oldEtcdConfigChecksum, cpn.Spec.Components.Etcd.Checksums.Config,
			"Etcd ConfigChecksum should not change when only PKI secret is updated")
		require.Equal(suite.T(), oldEtcdPKIChecksum, cpn.Spec.Components.Etcd.Checksums.PKI,
			"Etcd PKIChecksum should not change when only d8-pki secret is updated (it depends on cert-sans/encryption-algorithm)")
		require.Equal(suite.T(), oldAPIServerPKIChecksum, cpn.Spec.Components.KubeAPIServer.Checksums.PKI,
			"KubeAPIServer PKIChecksum should not change when only d8-pki secret is updated")
	})
}

// TestCertSANsChangeAffectsOnlyAPIServerPKI verifies that updating cert-sans in the config secret
// changes kube-apiserver PKIChecksum but not etcd PKIChecksum (cert-sans don't affect etcd certs).
// ConfigChecksums are unaffected because cert-sans is not a template dependency.
func (suite *ControllerTestSuite) TestCertSANsChangeAffectsOnlyAPIServerPKI() {
	suite.Run("cert-sans change should update only kube-apiserver PKIChecksum", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()
		oldEtcdConfig := cpn.Spec.Components.Etcd.Checksums.Config
		oldEtcdPKI := cpn.Spec.Components.Etcd.Checksums.PKI
		oldAPIServerConfig := cpn.Spec.Components.KubeAPIServer.Checksums.Config
		oldAPIServerPKI := cpn.Spec.Components.KubeAPIServer.Checksums.PKI

		configSecret := suite.getConfigSecret()
		configSecret.Data["cert-sans"] = []byte("new-san.example.com,10.0.0.1")
		require.NoError(suite.T(), suite.client.Update(suite.ctx, configSecret))

		suite.reconcile()

		cpn = suite.getControlPlaneNode()
		require.Equal(suite.T(), oldEtcdConfig, cpn.Spec.Components.Etcd.Checksums.Config,
			"Etcd ConfigChecksum must not change when cert-sans is updated")
		require.Equal(suite.T(), oldEtcdPKI, cpn.Spec.Components.Etcd.Checksums.PKI,
			"Etcd PKIChecksum must not change when cert-sans is updated (cert-sans only affects apiserver)")
		require.Equal(suite.T(), oldAPIServerConfig, cpn.Spec.Components.KubeAPIServer.Checksums.Config,
			"KubeAPIServer ConfigChecksum must not change when cert-sans is updated")
		require.NotEqual(suite.T(), oldAPIServerPKI, cpn.Spec.Components.KubeAPIServer.Checksums.PKI,
			"KubeAPIServer PKIChecksum must change when cert-sans is updated")
	})
}

// TestEncryptionAlgorithmChangeAffectsBothPKIChecksums verifies that updating encryption-algorithm
// changes PKIChecksum for both etcd and kube-apiserver, but leaves all ConfigChecksums untouched.
func (suite *ControllerTestSuite) TestEncryptionAlgorithmChangeAffectsBothPKIChecksums() {
	suite.Run("encryption-algorithm change should update PKIChecksum for etcd and kube-apiserver", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()
		oldEtcdConfig := cpn.Spec.Components.Etcd.Checksums.Config
		oldEtcdPKI := cpn.Spec.Components.Etcd.Checksums.PKI
		oldAPIServerConfig := cpn.Spec.Components.KubeAPIServer.Checksums.Config
		oldAPIServerPKI := cpn.Spec.Components.KubeAPIServer.Checksums.PKI
		oldKCMConfig := cpn.Spec.Components.KubeControllerManager.Checksums.Config
		oldSchedulerConfig := cpn.Spec.Components.KubeScheduler.Checksums.Config

		configSecret := suite.getConfigSecret()
		configSecret.Data["encryption-algorithm"] = []byte("RSA-4096")
		require.NoError(suite.T(), suite.client.Update(suite.ctx, configSecret))

		suite.reconcile()

		cpn = suite.getControlPlaneNode()
		require.NotEqual(suite.T(), oldEtcdPKI, cpn.Spec.Components.Etcd.Checksums.PKI,
			"Etcd PKIChecksum must change when encryption-algorithm is updated")
		require.NotEqual(suite.T(), oldAPIServerPKI, cpn.Spec.Components.KubeAPIServer.Checksums.PKI,
			"KubeAPIServer PKIChecksum must change when encryption-algorithm is updated")
		require.Equal(suite.T(), oldEtcdConfig, cpn.Spec.Components.Etcd.Checksums.Config,
			"Etcd ConfigChecksum must not change when encryption-algorithm is updated")
		require.Equal(suite.T(), oldAPIServerConfig, cpn.Spec.Components.KubeAPIServer.Checksums.Config,
			"KubeAPIServer ConfigChecksum must not change when encryption-algorithm is updated")
		require.Equal(suite.T(), oldKCMConfig, cpn.Spec.Components.KubeControllerManager.Checksums.Config,
			"KubeControllerManager ConfigChecksum must not change when encryption-algorithm is updated")
		require.Equal(suite.T(), oldSchedulerConfig, cpn.Spec.Components.KubeScheduler.Checksums.Config,
			"KubeScheduler ConfigChecksum must not change when encryption-algorithm is updated")
	})
}

// TestEtcdChecksumChangesOnManifestUpdate verifies that updating the etcd
// manifest in the config secret changes only the etcd checksum, leaving all other component checksums intact
func (suite *ControllerTestSuite) TestEtcdChecksumChangesOnManifestUpdate() {
	suite.Run("Etcd checksum should change when its manifest is updated; other checksums should not", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()
		oldEtcdChecksum := cpn.Spec.Components.Etcd.Checksums.Config
		oldAPIServerChecksum := cpn.Spec.Components.KubeAPIServer.Checksums.Config
		oldKCMChecksum := cpn.Spec.Components.KubeControllerManager.Checksums.Config
		oldSchedulerChecksum := cpn.Spec.Components.KubeScheduler.Checksums.Config

		configSecret := suite.getConfigSecret()
		configSecret.Data["etcd.yaml.tpl"] = append(configSecret.Data["etcd.yaml.tpl"], []byte("\n# updated")...)
		require.NoError(suite.T(), suite.client.Update(suite.ctx, configSecret))

		suite.reconcile()

		cpn = suite.getControlPlaneNode()
		require.NotEqual(suite.T(), oldEtcdChecksum, cpn.Spec.Components.Etcd.Checksums.Config,
			"Etcd checksum should change after manifest update")
		require.Equal(suite.T(), oldAPIServerChecksum, cpn.Spec.Components.KubeAPIServer.Checksums.Config,
			"KubeAPIServer checksum should not change when etcd manifest is updated")
		require.Equal(suite.T(), oldKCMChecksum, cpn.Spec.Components.KubeControllerManager.Checksums.Config,
			"KubeControllerManager checksum should not change when etcd manifest is updated")
		require.Equal(suite.T(), oldSchedulerChecksum, cpn.Spec.Components.KubeScheduler.Checksums.Config,
			"KubeScheduler checksum should not change when etcd manifest is updated")
	})
}

// TestAPIServerChecksumChangesOnExtraFileUpdate verifies that updating an extra
// file referenced only by kube-apiserver (audit-policy.yaml) changes the apiserver checksum while leaving the etcd checksum unchanged
func (suite *ControllerTestSuite) TestAPIServerChecksumChangesOnExtraFileUpdate() {
	suite.Run("KubeAPIServer checksum should change when its extra-file is updated; etcd should not", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		cpn := suite.getControlPlaneNode()
		oldAPIServerChecksum := cpn.Spec.Components.KubeAPIServer.Checksums.Config
		oldEtcdChecksum := cpn.Spec.Components.Etcd.Checksums.Config

		configSecret := suite.getConfigSecret()
		configSecret.Data["extra-file-audit-policy.yaml"] = []byte(
			"apiVersion: audit.k8s.io/v1\nkind: Policy\nrules:\n- level: RequestResponse\n",
		)
		require.NoError(suite.T(), suite.client.Update(suite.ctx, configSecret))

		suite.reconcile()

		cpn = suite.getControlPlaneNode()
		require.NotEqual(suite.T(), oldAPIServerChecksum, cpn.Spec.Components.KubeAPIServer.Checksums.Config,
			"KubeAPIServer checksum should change when extra-file-audit-policy.yaml is updated")
		require.Equal(suite.T(), oldEtcdChecksum, cpn.Spec.Components.Etcd.Checksums.Config,
			"Etcd checksum should not change when an apiserver extra-file is updated")
	})
}

// TestNodeDeletionCleansUpControlPlaneNode verifies that when a master Node is deleted from the cluster
// ControlPlaneNode is also deleted on the next reconciliation
func (suite *ControllerTestSuite) TestNodeDeletionCleansUpControlPlaneNode() {
	suite.Run("ControlPlaneNode should be deleted when its Node is deleted", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))
		suite.reconcile()

		// Verify CPN was created
		cpn := suite.getControlPlaneNode()
		require.NotEmpty(suite.T(), cpn.Name)

		// Delete the master Node
		node := &corev1.Node{}
		err := suite.client.Get(suite.ctx, client.ObjectKey{Name: testNodeName}, node)
		require.NoError(suite.T(), err)
		require.NoError(suite.T(), suite.client.Delete(suite.ctx, node))

		// Reconcile — controller should detect Node is gone and delete the CPN
		_, err = suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: testNodeName},
		})
		require.NoError(suite.T(), err)

		// CPN should no longer exist
		deletedCPN := &controlplanev1alpha1.ControlPlaneNode{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: testNodeName}, deletedCPN)
		require.True(suite.T(), apierrors.IsNotFound(err),
			"ControlPlaneNode should be NotFound after node deletion, got: %v", err)
	})
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.T().Failed() {
		return
	}

	suite.T().Log("Test failed, dumping resources:")
	for _, obj := range []client.ObjectList{
		&corev1.SecretList{},
		&corev1.NodeList{},
		&controlplanev1alpha1.ControlPlaneNodeList{},
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
		case "Node":
			node := &corev1.Node{}
			err = yaml.Unmarshal([]byte(manifest), node)
			require.NoError(suite.T(), err, "failed to unmarshal Node")
			objs = append(objs, node)
		case "ControlPlaneNode":
			cpn := &controlplanev1alpha1.ControlPlaneNode{}
			err = yaml.Unmarshal([]byte(manifest), cpn)
			require.NoError(suite.T(), err, "failed to unmarshal ControlPlaneNode")
			objs = append(objs, cpn)
		default:
			suite.T().Logf("unknown kind: %s", metaType.Kind)
		}
	}

	return objs
}
