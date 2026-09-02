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
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
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
	require.Empty(t, state.missingHosts)
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
		"API server hosts not found in cloud config: 10.0.0.3",
	)
	require.Equal(t, []string{"10.0.0.3"}, state.missingHosts)
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

func TestWaitForCloudConfigSecretReturnsExistingSecret(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	_, err := kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Create(
			t.Context(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "manual-bootstrap-for-master",
					Namespace:       cloudConfigSecretNamespace,
					ResourceVersion: "50",
				},
				Data: map[string][]byte{
					"apiserverEndpoints": []byte(
						"- 10.0.0.2:6443\n",
					),
					"cloud-config": []byte("#cloud-config\n"),
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	state, err := waitForCloudConfigSecret(
		t.Context(),
		kubeCl,
		"master",
		[]string{"10.0.0.2"},
		time.Second,
	)
	require.NoError(t, err)

	require.Equal(
		t,
		base64.StdEncoding.EncodeToString([]byte("#cloud-config\n")),
		state.cloudConfig,
	)
	require.Empty(t, state.missingHosts)
}

func TestWaitForCloudConfigSecretWaitsForUpdate(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	_, err := kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Create(
			t.Context(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manual-bootstrap-for-master",
					Namespace: cloudConfigSecretNamespace,
				},
				Data: map[string][]byte{
					"apiserverEndpoints": []byte(
						"- 10.0.0.2:6443\n",
					),
					"cloud-config": []byte("#old-cloud-config\n"),
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	type waitResult struct {
		state cloudConfigSecretState
		err   error
	}

	resultCh := make(chan waitResult, 1)

	go func() {
		state, err := waitForCloudConfigSecret(
			t.Context(),
			kubeCl,
			"master",
			[]string{"10.0.0.2", "10.0.0.3"},
			time.Second,
		)

		resultCh <- waitResult{
			state: state,
			err:   err,
		}
	}()

	time.Sleep(50 * time.Millisecond)

	secret, err := kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Get(
			t.Context(),
			"manual-bootstrap-for-master",
			metav1.GetOptions{},
		)
	require.NoError(t, err)

	secret.Data["apiserverEndpoints"] = []byte(
		"- 10.0.0.2:6443\n- 10.0.0.3:6443\n",
	)
	secret.Data["cloud-config"] = []byte("#new-cloud-config\n")

	_, err = kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Update(
			t.Context(),
			secret,
			metav1.UpdateOptions{},
		)
	require.NoError(t, err)

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Equal(
			t,
			base64.StdEncoding.EncodeToString(
				[]byte("#new-cloud-config\n"),
			),
			result.state.cloudConfig,
		)
		require.Empty(t, result.state.missingHosts)

	case <-time.After(2 * time.Second):
		t.Fatal("waitForCloudConfigSecret did not return after Secret update")
	}
}

func TestWaitForCloudConfigSecretReportsTimeoutState(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	_, err := kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Create(
			t.Context(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "manual-bootstrap-for-master",
					Namespace:       cloudConfigSecretNamespace,
					ResourceVersion: "51",
				},
				Data: map[string][]byte{
					"apiserverEndpoints": []byte(
						"- 10.0.0.2:6443\n",
					),
					"cloud-config": []byte("#cloud-config\n"),
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	state, err := waitForCloudConfigSecret(
		t.Context(),
		kubeCl,
		"master",
		[]string{"10.0.0.2", "10.0.0.3"},
		200*time.Millisecond,
	)

	require.ErrorContains(t, err, "timeout after 200ms")
	require.ErrorContains(
		t,
		err,
		"manual-bootstrap-for-master",
	)
	require.ErrorContains(t, err, "10.0.0.3")
	require.Equal(t, []string{"10.0.0.3"}, state.missingHosts)
	require.Equal(t, []string{"10.0.0.2:6443"}, state.endpoints)
}

func TestWaitForCloudConfigSecretWaitsForCreation(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	type waitResult struct {
		state cloudConfigSecretState
		err   error
	}

	resultCh := make(chan waitResult, 1)

	go func() {
		state, err := waitForCloudConfigSecret(
			t.Context(),
			kubeCl,
			"master",
			[]string{"10.0.0.2"},
			time.Second,
		)

		resultCh <- waitResult{
			state: state,
			err:   err,
		}
	}()

	// Give the waiter time to perform the initial List and start Watch.
	time.Sleep(50 * time.Millisecond)

	_, err := kubeCl.CoreV1().
		Secrets(cloudConfigSecretNamespace).
		Create(
			t.Context(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manual-bootstrap-for-master",
					Namespace: cloudConfigSecretNamespace,
				},
				Data: map[string][]byte{
					"apiserverEndpoints": []byte(
						"- 10.0.0.2:6443\n",
					),
					"cloud-config": []byte("#created-cloud-config\n"),
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Equal(
			t,
			[]string{"10.0.0.2:6443"},
			result.state.endpoints,
		)
		require.Empty(t, result.state.missingHosts)
		require.Equal(
			t,
			base64.StdEncoding.EncodeToString(
				[]byte("#created-cloud-config\n"),
			),
			result.state.cloudConfig,
		)

	case <-time.After(2 * time.Second):
		t.Fatal(
			"waitForCloudConfigSecret did not return after Secret creation",
		)
	}
}
