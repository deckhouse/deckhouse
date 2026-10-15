// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	klient "github.com/flant/kube-client/client"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

func TestInspectCloudConfigSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "42",
		},
		Data: map[string][]byte{
			"apiserverEndpoints": []byte(
				"- 10.0.0.2:6443\n- 10.0.0.3:6443\n",
			),
			"cloud-config": []byte("#cloud-config\n"),
		},
	}

	state, err := inspectCloudConfigSecret(
		secret,
		[]string{"10.0.0.3", "10.0.0.2"},
	)
	require.NoError(t, err)

	require.Equal(t, "42", state.resourceVersion)
	require.Equal(
		t,
		[]string{"10.0.0.2:6443", "10.0.0.3:6443"},
		state.endpoints,
	)
	require.Equal(
		t,
		base64.StdEncoding.EncodeToString([]byte("#cloud-config\n")),
		state.cloudConfig,
	)
}

func TestInspectCloudConfigSecretReportsMissingHosts(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "43",
		},
		Data: map[string][]byte{
			"apiserverEndpoints": []byte("- 10.0.0.2:6443\n"),
			"cloud-config":       []byte("#cloud-config\n"),
		},
	}

	state, err := inspectCloudConfigSecret(
		secret,
		[]string{"10.0.0.2", "10.0.0.3"},
	)

	require.ErrorContains(
		t,
		err,
		"API server hosts not found in apiserverEndpoints: 10.0.0.3",
	)
	require.Equal(t, "43", state.resourceVersion)
}

func TestInspectCloudConfigSecretRejectsEmptyCloudConfig(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"apiserverEndpoints": []byte("- 10.0.0.2:6443\n"),
		},
	}

	_, err := inspectCloudConfigSecret(
		secret,
		[]string{"10.0.0.2"},
	)

	require.ErrorContains(t, err, "cloud-config is missing or empty")
}

func TestInspectCloudConfigSecretReportsAllHostsWhenEndpointsMissing(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"cloud-config": []byte("#cloud-config\n"),
		},
	}

	_, err := inspectCloudConfigSecret(
		secret,
		[]string{"10.0.0.3", "10.0.0.2"},
	)

	require.ErrorContains(t, err, "10.0.0.2, 10.0.0.3")
}

func newCloudConfigSecret(name, resourceVersion, endpoints, cloudConfig string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       cloudConfigSecretNamespace,
			ResourceVersion: resourceVersion,
		},
		Data: map[string][]byte{
			"apiserverEndpoints": []byte(endpoints),
			"cloud-config":       []byte(cloudConfig),
		},
	}
}

func newFakeKubeClient(t *testing.T, secrets ...*corev1.Secret) (*client.KubernetesClient, *k8sfake.Clientset) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClient()

	for _, secret := range secrets {
		_, err := kubeCl.CoreV1().
			Secrets(cloudConfigSecretNamespace).
			Create(t.Context(), secret, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	return kubeCl, clientset
}

func TestWaitForCloudConfigSecretReturnsExistingSecret(t *testing.T) {
	kubeCl, _ := newFakeKubeClient(t, newCloudConfigSecret(
		"manual-bootstrap-for-master", "50", "- 10.0.0.2:6443\n", "#cloud-config\n",
	))

	cloudConfig, err := waitForCloudConfigSecret(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		[]string{"10.0.0.2"},
		1,
		time.Hour,
	)
	require.NoError(t, err)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("#cloud-config\n")), cloudConfig)
}

func TestGetCloudConfigReturnsExistingCloudConfig(t *testing.T) {
	kubeCl, _ := newFakeKubeClient(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manual-bootstrap-for-master",
			Namespace: cloudConfigSecretNamespace,
		},
		Data: map[string][]byte{
			"cloud-config": []byte("#cloud-config\n"),
		},
	})

	cloudConfig, err := GetCloudConfig(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		false,
	)
	require.NoError(t, err)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("#cloud-config\n")), cloudConfig)
}

func TestWaitForCloudConfigSecretWaitsForFreshSecret(t *testing.T) {
	kubeCl, clientset := newFakeKubeClient(t, newCloudConfigSecret(
		"manual-bootstrap-for-master", "", "- 10.0.0.2:6443\n- 10.0.0.3:6443\n", "#new-cloud-config\n",
	))

	stale := newCloudConfigSecret("manual-bootstrap-for-master", "", "- 10.0.0.2:6443\n", "#old-cloud-config\n")

	gets := 0
	clientset.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets == 1 {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, stale.Name)
		}

		if gets == 2 {
			return true, stale, nil
		}

		return false, nil, nil
	})

	cloudConfig, err := waitForCloudConfigSecret(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		[]string{"10.0.0.2", "10.0.0.3"},
		5,
		time.Millisecond,
	)
	require.NoError(t, err)
	require.Equal(t, 3, gets)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("#new-cloud-config\n")), cloudConfig)
}

func TestWaitForCloudConfigSecretReportsLastState(t *testing.T) {
	kubeCl, _ := newFakeKubeClient(t, newCloudConfigSecret(
		"manual-bootstrap-for-master", "51", "- 10.0.0.2:6443\n", "#cloud-config\n",
	))

	_, err := waitForCloudConfigSecret(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		[]string{"10.0.0.2", "10.0.0.3"},
		2,
		time.Millisecond,
	)

	require.ErrorContains(t, err, "d8-cloud-instance-manager/manual-bootstrap-for-master")
	require.ErrorContains(t, err, `resourceVersion="51"`)
	require.ErrorContains(t, err, "endpoints=[10.0.0.2:6443]")
	require.ErrorContains(t, err, "API server hosts not found in apiserverEndpoints: 10.0.0.3")
}

func TestWaitForCloudConfigSecretIgnoresOtherNodeGroupSecret(t *testing.T) {
	kubeCl, _ := newFakeKubeClient(t, newCloudConfigSecret(
		"manual-bootstrap-for-worker", "", "- 10.0.0.2:6443\n", "#worker-cloud-config\n",
	))

	_, err := waitForCloudConfigSecret(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		[]string{"10.0.0.2"},
		1,
		time.Hour,
	)

	require.True(t, apierrors.IsNotFound(err), "unexpected error: %v", err)
}

func TestWaitForCloudConfigSecretStopsOnContextCancel(t *testing.T) {
	kubeCl, _ := newFakeKubeClient(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := waitForCloudConfigSecret(
		ctx,
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		nil,
		3,
		time.Millisecond,
	)

	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitForCloudConfigSecretStopsOnPermanentAuthError(t *testing.T) {
	kubeCl, clientset := newFakeKubeClient(t)

	gets := 0
	clientset.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		gets++
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "manual-bootstrap-for-master", nil)
	})

	_, err := waitForCloudConfigSecret(
		kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeOwnCredentials),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		nil,
		3,
		time.Millisecond,
	)

	require.True(t, apierrors.IsForbidden(err), "unexpected error: %v", err)
	require.Equal(t, 1, gets)
}

func TestWaitForCloudConfigSecretRejectsEmptyExpectedHost(t *testing.T) {
	kubeCl, clientset := newFakeKubeClient(t)

	gets := 0
	clientset.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		gets++
		return false, nil, nil
	})

	_, err := waitForCloudConfigSecret(
		t.Context(),
		kubernetes.NewSimpleKubeClientGetter(kubeCl),
		"master",
		[]string{"10.0.0.2", " "},
		225,
		time.Hour,
	)

	require.ErrorContains(t, err, "expected API server host is empty")
	require.Zero(t, gets)
}
