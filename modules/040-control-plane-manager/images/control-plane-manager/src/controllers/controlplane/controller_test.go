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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/pkg/constants"
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

type mockManifestGenerator struct{}

// mockManifestGenerator returns manifests for testing (like kubeadm-generated manifests)
func (m *mockManifestGenerator) GenerateManifest(componentName string, tmpDir string) ([]byte, error) {
	// for kube-apiserver, return manifest with audit-policy.yaml referenced
	if componentName == "kube-apiserver" {
		return []byte(`apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    image: test:latest
    command:
    - kube-apiserver
    - --audit-policy-file=/etc/kubernetes/deckhouse/extra-files/audit-policy.yaml
    - --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
    volumeMounts:
    - mountPath: /etc/kubernetes/deckhouse/extra-files
      name: extra-files
  volumes:
  - hostPath:
      path: /etc/kubernetes/deckhouse/extra-files
    name: extra-files
`), nil
	}
	// for other components, return simple manifest
	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: kube-system
spec:
  containers:
  - name: %s
    image: test:latest
    command:
    - %s
    volumeMounts:
    - mountPath: /etc/kubernetes/pki
      name: certs
  volumes:
  - hostPath:
      path: /etc/kubernetes/pki
    name: certs
`, componentName, componentName, componentName)), nil
}

func (m *mockManifestGenerator) GenerateCertificates(componentName string, tmpDir string) error {
	pkiDir := filepath.Join(tmpDir, constants.RelativePkiDir)
	etcdPkiDir := filepath.Join(pkiDir, "etcd")
	if err := os.MkdirAll(etcdPkiDir, 0o700); err != nil {
		return err
	}

	// Generate mock certificates based on component (not depends on CA)
	switch componentName {
	case "etcd":
		for _, certName := range []string{"peer", "server", "healthcheck-client"} {
			certPath := filepath.Join(etcdPkiDir, certName+".crt")
			keyPath := filepath.Join(etcdPkiDir, certName+".key")

			if err := os.WriteFile(certPath, []byte("mock-cert-"+certName), 0o600); err != nil {
				return err
			}
			if err := os.WriteFile(keyPath, []byte("mock-key-"+certName), 0o600); err != nil {
				return err
			}
		}
	case "kube-apiserver":
		for _, certName := range []string{"apiserver", "apiserver-kubelet-client", "apiserver-etcd-client"} {
			certPath := filepath.Join(pkiDir, certName+".crt")
			keyPath := filepath.Join(pkiDir, certName+".key")

			if err := os.WriteFile(certPath, []byte("mock-cert-"+certName), 0o600); err != nil {
				return err
			}
			if err := os.WriteFile(keyPath, []byte("mock-key-"+certName), 0o600); err != nil {
				return err
			}
		}
		// Generate front-proxy-client (depends on CA) cert based on front-proxy-ca from PKI dir for TestApiserverChecksumChangesOnFrontProxyCAUpdate
		// Read front-proxy-ca to simulate generating client cert from CA
		frontProxyCACert := filepath.Join(pkiDir, "front-proxy-ca.crt")
		frontProxyCAKey := filepath.Join(pkiDir, "front-proxy-ca.key")

		caContent, err := os.ReadFile(frontProxyCACert)
		if err != nil {
			return fmt.Errorf("failed to read front-proxy-ca.crt: %w", err)
		}
		caKeyContent, err := os.ReadFile(frontProxyCAKey)
		if err != nil {
			return fmt.Errorf("failed to read front-proxy-ca.key: %w", err)
		}

		// Generate client cert with content that depends on CA
		// This simulates regeneration with new content when CA changes
		frontProxyCert := filepath.Join(pkiDir, "front-proxy-client.crt")
		frontProxyKey := filepath.Join(pkiDir, "front-proxy-client.key")
		clientCertContent := fmt.Sprintf("mock-cert-front-proxy-client-ca:%x-cakey:%x",
			sha256.Sum256(caContent), sha256.Sum256(caKeyContent))

		if err := os.WriteFile(frontProxyCert, []byte(clientCertContent), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(frontProxyKey, []byte("mock-key-front-proxy-client"), 0o600); err != nil {
			return err
		}
	}

	return nil
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
		client:            suite.client,
		manifestGenerator: &mockManifestGenerator{},
	}
}

func (suite *ControllerTestSuite) TestReconcileCreatesConfiguration() {
	suite.Run("ControlPlaneConfiguration should be created with all checksums", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		_, err := suite.controller.Reconcile(
			suite.ctx,
			reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: constants.ControlPlaneConfigurationName,
				},
			},
		)

		require.NoError(suite.T(), err)

		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err, "ControlPlaneConfiguration should exist")

		// Verify all checksums are present and non-empty
		require.NotEmpty(suite.T(), cpc.Spec.PKIChecksum, "PKIChecksum should not be empty")
		require.NotNil(suite.T(), cpc.Spec.Components, "Components should not be nil")

		require.NotNil(suite.T(), cpc.Spec.Components.Etcd, "Etcd should not be nil")
		require.NotEmpty(suite.T(), cpc.Spec.Components.Etcd.Checksum, "Etcd checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeAPIServer, "KubeAPIServer should not be nil")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeAPIServer.Checksum, "KubeAPIServer checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeControllerManager, "KubeControllerManager should not be nil")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeControllerManager.Checksum, "KubeControllerManager checksum should not be empty")

		require.NotNil(suite.T(), cpc.Spec.Components.KubeScheduler, "KubeScheduler should not be nil")
		require.NotEmpty(suite.T(), cpc.Spec.Components.KubeScheduler.Checksum, "KubeScheduler checksum should not be empty")

		suite.T().Logf("PKI Checksum: %s", cpc.Spec.PKIChecksum)
		suite.T().Logf("Etcd Checksum: %s", cpc.Spec.Components.Etcd.Checksum)
		suite.T().Logf("KubeAPIServer Checksum: %s", cpc.Spec.Components.KubeAPIServer.Checksum)
		suite.T().Logf("KubeControllerManager Checksum: %s", cpc.Spec.Components.KubeControllerManager.Checksum)
		suite.T().Logf("KubeScheduler Checksum: %s", cpc.Spec.Components.KubeScheduler.Checksum)
	})
}

func (suite *ControllerTestSuite) TestPKIChecksumChanges() {
	suite.Run("PKIChecksum should change when PKI secret is updated", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		// Trigger first reconcile (initial state)
		_, err := suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		// Get initial checksum
		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)
		oldPKIChecksum := cpc.Spec.PKIChecksum

		// Update PKI secret
		pkiSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.PkiSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, pkiSecret)
		require.NoError(suite.T(), err)

		// Modify CA certificate
		pkiSecret.Data["ca.crt"] = []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5FVyBDQSBDRVJUCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=")
		err = suite.client.Update(suite.ctx, pkiSecret)
		require.NoError(suite.T(), err)

		// Trigger second reconcile
		_, err = suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		// Get updated checksum
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)

		// PKI checksum should have changed
		require.NotEqual(suite.T(), oldPKIChecksum, cpc.Spec.PKIChecksum,
			"PKI checksum should change after secret update")

		suite.T().Logf("Old PKI Checksum: %s", oldPKIChecksum)
		suite.T().Logf("New PKI Checksum: %s", cpc.Spec.PKIChecksum)
	})
}

func (suite *ControllerTestSuite) TestApiserverChecksumChangesOnFrontProxyCAUpdate() {
	suite.Run("KubeAPIServer checksum should change when front-proxy-ca is updated", func() {
		suite.setupController(suite.fetchTestFileData("basic-config.yaml"))

		// Trigger first reconcile (initial state)
		_, err := suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)
		oldApiserverChecksum := cpc.Spec.Components.KubeAPIServer.Checksum

		pkiSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.PkiSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, pkiSecret)
		require.NoError(suite.T(), err)

		// Update PKI secret - modify front-proxy-ca certificate
		// This simulates when front-proxy CA certificate is rotated
		// GenerateCertificates("kube-apiserver") uses these to generate front-proxy-client.crt
		pkiSecret.Data["front-proxy-ca.crt"] = []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5FVyBGUk9OVCBQUk9YWSBDQSBDRVJUCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=")
		pkiSecret.Data["front-proxy-ca.key"] = []byte("LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk5FVyBGUk9OVCBQUk9YWSBDQSBLRVkKLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQ==")
		err = suite.client.Update(suite.ctx, pkiSecret)
		require.NoError(suite.T(), err)

		// Trigger second reconcile
		// During reconcile, GenerateCertificates("kube-apiserver", tmpDir) will be called
		// which will regenerate front-proxy-client.crt based on the updated front-proxy-ca
		_, err = suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		// Get updated checksum
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)

		// KubeAPIServer checksum should have changed because:
		// 1. front-proxy-ca.crt/key was updated in PKI secret
		// 2. GenerateCertificates("kube-apiserver") regenerated front-proxy-client.crt based on new CA
		// 3. --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt references the regenerated cert
		require.NotEqual(suite.T(), oldApiserverChecksum, cpc.Spec.Components.KubeAPIServer.Checksum,
			"KubeAPIServer checksum should change when front-proxy-ca is updated and client cert is regenerated")

		suite.T().Logf("Old KubeAPIServer Checksum: %s", oldApiserverChecksum)
		suite.T().Logf("New KubeAPIServer Checksum: %s", cpc.Spec.Components.KubeAPIServer.Checksum)
	})
}

func (suite *ControllerTestSuite) TestSyncSecretToTmp() {
	suite.Run("syncSecretToTmp should create correct directory structure", func() {
		objs := suite.fetchTestFileData("basic-config.yaml")
		suite.setupController(objs)

		cmpSecret := &corev1.Secret{}
		err := suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.ControlPlaneManagerConfigSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, cmpSecret)
		require.NoError(suite.T(), err)

		pkiSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.PkiSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, pkiSecret)
		require.NoError(suite.T(), err)

		tmpDir, err := os.MkdirTemp("", "control-plane-test-")
		require.NoError(suite.T(), err)
		defer os.RemoveAll(tmpDir)

		err = syncSecretToTmp(cmpSecret, tmpDir)
		require.NoError(suite.T(), err)

		err = syncSecretToTmp(pkiSecret, tmpDir)
		require.NoError(suite.T(), err)

		kubeadmDir := filepath.Join(tmpDir, constants.RelativeKubeadmDir)
		patchesDir := filepath.Join(tmpDir, constants.RelativePatchesDir)
		extraFilesDir := filepath.Join(tmpDir, constants.RelativeExtraFilesDir)
		pkiDir := filepath.Join(tmpDir, constants.RelativePkiDir)

		require.DirExists(suite.T(), kubeadmDir, "Kubeadm directory should exist")
		require.DirExists(suite.T(), patchesDir, "Patches directory should exist")
		require.DirExists(suite.T(), extraFilesDir, "Extra files directory should exist")
		require.DirExists(suite.T(), pkiDir, "PKI directory should exist")

		kubeadmConfigPath := filepath.Join(kubeadmDir, "config.yaml")
		require.FileExists(suite.T(), kubeadmConfigPath, "Kubeadm config should exist")

		content, err := os.ReadFile(kubeadmConfigPath)
		require.NoError(suite.T(), err)
		require.Contains(suite.T(), string(content), "apiVersion: kubeadm.k8s.io/v1beta3",
			"Kubeadm config should be valid")

		auditPolicyPath := filepath.Join(extraFilesDir, "audit-policy.yaml")
		require.FileExists(suite.T(), auditPolicyPath, "Audit policy should exist")

		content, err = os.ReadFile(auditPolicyPath)
		require.NoError(suite.T(), err)
		require.Contains(suite.T(), string(content), "apiVersion: audit.k8s.io/v1",
			"Audit policy should be valid")

		pkiFiles := []string{"ca.crt", "ca.key", "front-proxy-ca.key", "front-proxy-ca.crt", "etcd/ca.crt", "etcd/ca.key"}
		for _, file := range pkiFiles {
			pkiFilePath := filepath.Join(pkiDir, file)
			require.FileExists(suite.T(), pkiFilePath, fmt.Sprintf("PKI file %s should exist", file))

			content, err := os.ReadFile(pkiFilePath)
			require.NoError(suite.T(), err)
			require.NotEmpty(suite.T(), content, fmt.Sprintf("PKI file %s should not be empty", file))

			suite.T().Logf("PKI file %s validated successfully (size: %d bytes)", file, len(content))
		}

		suite.T().Log("Directory structure validated successfully")
	})
}

func (suite *ControllerTestSuite) TestReferencedFilesAffectChecksum() {
	suite.Run("Component checksum should change when referenced files change", func() {
		objs := suite.fetchTestFileData("basic-config.yaml")
		suite.setupController(objs)

		_, err := suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		cpc := &controlplanev1alpha1.ControlPlaneConfiguration{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)

		oldApiServerChecksum := cpc.Spec.Components.KubeAPIServer.Checksum
		oldEtcdChecksum := cpc.Spec.Components.Etcd.Checksum

		configSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.ControlPlaneManagerConfigSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, configSecret)
		require.NoError(suite.T(), err)

		configSecret.Data["extra-file-audit-policy.yaml"] = []byte("YXBpVmVyc2lvbjogYXVkaXQuazhzLmlvL3YxCmtpbmQ6IFBvbGljeQpydWxlczoKLSBsZXZlbDogUmVxdWVzdFJlc3BvbnNlCg==")
		err = suite.client.Update(suite.ctx, configSecret)
		require.NoError(suite.T(), err)

		_, err = suite.controller.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: constants.ControlPlaneConfigurationName},
		})
		require.NoError(suite.T(), err)

		err = suite.client.Get(suite.ctx, client.ObjectKey{Name: constants.ControlPlaneConfigurationName}, cpc)
		require.NoError(suite.T(), err)

		require.NotEqual(suite.T(), oldApiServerChecksum, cpc.Spec.Components.KubeAPIServer.Checksum,
			"KubeAPIServer checksum should change when referenced file (audit-policy.yaml) changes")

		require.Equal(suite.T(), oldEtcdChecksum, cpc.Spec.Components.Etcd.Checksum,
			"Etcd checksum should not change when unrelated file changes")

		suite.T().Logf("Old KubeAPIServer checksum: %s", oldApiServerChecksum)
		suite.T().Logf("New KubeAPIServer checksum: %s", cpc.Spec.Components.KubeAPIServer.Checksum)
		suite.T().Logf("Etcd checksum (unchanged): %s", cpc.Spec.Components.Etcd.Checksum)
	})
}

func (suite *ControllerTestSuite) TestChecksumCalculation() {
	suite.Run("Checksums should be stable and change only when data changes", func() {
		objs := suite.fetchTestFileData("basic-config.yaml")
		suite.setupController(objs)

		pkiSecret := &corev1.Secret{}
		err := suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.PkiSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, pkiSecret)
		require.NoError(suite.T(), err)

		checksum1, err := calculatePKIChecksum(pkiSecret)
		require.NoError(suite.T(), err)

		checksum2, err := calculatePKIChecksum(pkiSecret)
		require.NoError(suite.T(), err)

		require.Equal(suite.T(), checksum1, checksum2,
			"Checksums should be stable for same data")

		pkiSecret.Data["ca.crt"] = []byte("MODIFIED_DATA")

		checksum3, err := calculatePKIChecksum(pkiSecret)
		require.NoError(suite.T(), err)

		require.NotEqual(suite.T(), checksum1, checksum3,
			"Checksum should change when data changes")

		suite.T().Logf("Original checksum: %s", checksum1)
		suite.T().Logf("Modified checksum: %s", checksum3)
	})
}

func (suite *ControllerTestSuite) TestCertificateGeneration() {
	suite.Run("Certificates should be generated before manifest generation", func() {
		objs := suite.fetchTestFileData("basic-config.yaml")
		suite.setupController(objs)

		cmpSecret := &corev1.Secret{}
		err := suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.ControlPlaneManagerConfigSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, cmpSecret)
		require.NoError(suite.T(), err)

		pkiSecret := &corev1.Secret{}
		err = suite.client.Get(suite.ctx, client.ObjectKey{
			Name:      constants.PkiSecretName,
			Namespace: constants.KubeSystemNamespace,
		}, pkiSecret)
		require.NoError(suite.T(), err)

		tmpDir, err := os.MkdirTemp("", "control-plane-cert-test-")
		require.NoError(suite.T(), err)
		defer os.RemoveAll(tmpDir)

		// Sync secrets to tmpDir
		err = syncSecretToTmp(cmpSecret, tmpDir)
		require.NoError(suite.T(), err)

		err = syncSecretToTmp(pkiSecret, tmpDir)
		require.NoError(suite.T(), err)

		generator := &mockManifestGenerator{}

		// Test etcd certificate generation
		err = generator.GenerateCertificates("etcd", tmpDir)
		require.NoError(suite.T(), err)

		etcdPkiDir := filepath.Join(tmpDir, constants.RelativePkiDir, "etcd")

		// Verify etcd certificates were created
		etcdCerts := []string{"peer.crt", "peer.key", "server.crt", "server.key", "healthcheck-client.crt", "healthcheck-client.key"}
		for _, certFile := range etcdCerts {
			certPath := filepath.Join(etcdPkiDir, certFile)
			require.FileExists(suite.T(), certPath, fmt.Sprintf("etcd certificate %s should exist", certFile))

			content, err := os.ReadFile(certPath)
			require.NoError(suite.T(), err)
			require.NotEmpty(suite.T(), content, fmt.Sprintf("etcd certificate %s should not be empty", certFile))

			suite.T().Logf("etcd certificate %s created successfully (size: %d bytes)", certFile, len(content))
		}

		// Test kube-apiserver certificate generation
		err = generator.GenerateCertificates("kube-apiserver", tmpDir)
		require.NoError(suite.T(), err)

		pkiDir := filepath.Join(tmpDir, constants.RelativePkiDir)

		// Verify apiserver certificates were created
		apiserverCerts := []string{
			"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
			"apiserver-etcd-client.crt", "apiserver-etcd-client.key",
			"front-proxy-client.crt", "front-proxy-client.key",
		}
		for _, certFile := range apiserverCerts {
			certPath := filepath.Join(pkiDir, certFile)
			require.FileExists(suite.T(), certPath, fmt.Sprintf("apiserver certificate %s should exist", certFile))

			content, err := os.ReadFile(certPath)
			require.NoError(suite.T(), err)
			require.NotEmpty(suite.T(), content, fmt.Sprintf("apiserver certificate %s should not be empty", certFile))

			suite.T().Logf("apiserver certificate %s created successfully (size: %d bytes)", certFile, len(content))
		}

		// Test that kube-controller-manager and kube-scheduler don't generate certs
		err = generator.GenerateCertificates("kube-controller-manager", tmpDir)
		require.NoError(suite.T(), err, "kube-controller-manager should not fail even without certs")

		err = generator.GenerateCertificates("kube-scheduler", tmpDir)
		require.NoError(suite.T(), err, "kube-scheduler should not fail even without certs")

		suite.T().Log("All certificate generation tests passed")
	})
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if !suite.T().Failed() {
		return
	}

	suite.T().Log("Test failed, dumping resources:")
	for _, obj := range []client.ObjectList{
		&corev1.SecretList{},
		&controlplanev1alpha1.ControlPlaneConfigurationList{},
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
