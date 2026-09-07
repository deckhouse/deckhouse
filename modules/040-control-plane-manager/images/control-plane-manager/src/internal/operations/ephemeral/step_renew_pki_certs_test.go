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

package ephemeral

import (
	"context"
	"testing"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace = "vcp-golden"
	testVCPName   = "golden"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, controlplanev1alpha1.AddToScheme(scheme))
	return scheme
}

func newStepExecutor(t *testing.T, objects ...client.Object) *StepExecutor {
	t.Helper()
	scheme := newTestScheme(t)
	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...)
	return &StepExecutor{
		client:         builder.Build(),
		tenantIdentity: tenantIdentity{Namespace: testNamespace, VCPName: testVCPName},
	}
}

// vcpObject builds the VirtualControlPlane the executor reads its networking from.
func vcpObject(serviceSubnetCIDR, podSubnetCIDR, nodeCIDRPrefix, clusterDomain string) *controlplanev1alpha1.VirtualControlPlane {
	return &controlplanev1alpha1.VirtualControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVCPName,
			Namespace: testNamespace,
		},
		Spec: controlplanev1alpha1.VirtualControlPlaneSpec{
			KubernetesVersion: "1.32",
			Networking: controlplanev1alpha1.VirtualControlPlaneNetworking{
				ServiceSubnetCIDR:       serviceSubnetCIDR,
				PodSubnetCIDR:           podSubnetCIDR,
				PodSubnetNodeCIDRPrefix: nodeCIDRPrefix,
				ClusterDomain:           clusterDomain,
			},
		},
	}
}

func defaultVCPObject() *controlplanev1alpha1.VirtualControlPlane {
	return vcpObject("10.96.0.0/12", "10.244.0.0/16", "24", "cluster.virtual")
}

func configSecretObject(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualResourceName(constants.VirtualRenderedConfigSecretName, testVCPName),
			Namespace: testNamespace,
		},
		Data: data,
	}
}

func TestLoadTenantPKIConfig(t *testing.T) {
	t.Run("networking from the VCP, cert-sans and encryption-algorithm from the config secret", func(t *testing.T) {
		secret := configSecretObject(map[string][]byte{
			constants.SecretKeyCertSANs:            []byte("1.2.3.4,api.example.com"),
			constants.SecretKeyEncryptionAlgorithm: []byte("RSA-2048"),
		})

		e := newStepExecutor(t, defaultVCPObject(), secret)

		cfg, err := e.loadTenantPKIConfig(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "cluster.virtual", cfg.ClusterDomain)
		assert.Equal(t, "10.96.0.0/12", cfg.ServiceSubnetCIDR)
		assert.Equal(t, []string{"1.2.3.4", "api.example.com"}, cfg.APIServerCertSANs)
		assert.Equal(t, "RSA-2048", cfg.EncryptionAlgorithm)
	})

	t.Run("custom networking on the VCP flows through", func(t *testing.T) {
		vcp := vcpObject("192.168.0.0/16", "10.111.0.0/16", "25", "tenant.example")

		e := newStepExecutor(t, vcp, configSecretObject(nil))

		cfg, err := e.loadTenantPKIConfig(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "tenant.example", cfg.ClusterDomain)
		assert.Equal(t, "192.168.0.0/16", cfg.ServiceSubnetCIDR)
		assert.Empty(t, cfg.APIServerCertSANs)
		assert.Empty(t, cfg.EncryptionAlgorithm)
	})

	// clusterDomain has no Go-side fallback: the CRD default guarantees a value on anything that
	// came through the API server, so the object's value is passed through verbatim.
	t.Run("clusterDomain is passed through verbatim", func(t *testing.T) {
		vcp := vcpObject("10.96.0.0/12", "10.244.0.0/16", "24", "tenant.internal")

		e := newStepExecutor(t, vcp, configSecretObject(nil))

		cfg, err := e.loadTenantPKIConfig(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "tenant.internal", cfg.ClusterDomain)
	})

	// The config secret only carries optional overrides now, so its absence must not fail the step.
	t.Run("missing config secret is not an error", func(t *testing.T) {
		e := newStepExecutor(t, defaultVCPObject())

		cfg, err := e.loadTenantPKIConfig(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "10.96.0.0/12", cfg.ServiceSubnetCIDR)
		assert.Empty(t, cfg.APIServerCertSANs)
	})

	t.Run("missing VCP errors", func(t *testing.T) {
		e := newStepExecutor(t, configSecretObject(nil))

		_, err := e.loadTenantPKIConfig(context.Background())
		assert.Error(t, err)
	})

}
