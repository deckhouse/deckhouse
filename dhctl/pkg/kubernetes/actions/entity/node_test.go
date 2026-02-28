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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestExtractPackagesProxyAddressesFromBootstrapSecret(t *testing.T) {
	t.Run("uses structured addresses from secret", func(t *testing.T) {
		got, err := extractPackagesProxyAddressesFromBootstrapSecret(map[string][]byte{
			"packagesProxyAddresses": []byte("- 10.241.32.22:4219\n- 10.241.36.14:4219\n"),
		})
		require.NoError(t, err)
		require.Equal(t, []string{"10.241.32.22:4219", "10.241.36.14:4219"}, got)
	})

	t.Run("returns empty when structured key is missing", func(t *testing.T) {
		got, err := extractPackagesProxyAddressesFromBootstrapSecret(map[string][]byte{
			"cloud-config": []byte(`#!/bin/bash
export PACKAGES_PROXY_ADDRESSES="10.241.32.22:4219,10.241.36.14:4219"`),
		})
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("returns error for malformed structured addresses", func(t *testing.T) {
		_, err := extractPackagesProxyAddressesFromBootstrapSecret(map[string][]byte{
			"packagesProxyAddresses": []byte(`{"invalid":"yaml-list"`),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "unmarshal packagesProxyAddresses")
	})

	t.Run("handles empty structured addresses list", func(t *testing.T) {
		got, err := extractPackagesProxyAddressesFromBootstrapSecret(map[string][]byte{
			"packagesProxyAddresses": []byte("[]"),
		})
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("handles null structured addresses list", func(t *testing.T) {
		got, err := extractPackagesProxyAddressesFromBootstrapSecret(map[string][]byte{
			"packagesProxyAddresses": []byte("null"),
		})
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestGetCloudConfigWithOptions_ExcludedPackagesProxyIPStillPresent(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manual-bootstrap-for-master",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: map[string][]byte{
			"cloud-config": []byte(`#!/bin/bash
export PACKAGES_PROXY_ADDRESSES="10.241.32.22:4219,10.241.36.14:4219"`),
			"packagesProxyAddresses": []byte("- 10.241.32.22:4219\n- 10.241.36.14:4219\n"),
		},
	}

	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(context.Background(), secret, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = GetCloudConfigWithOptions(
		context.Background(),
		kubeCl,
		"master",
		false,
		log.GetDefaultLogger(),
		CloudConfigOptions{
			ExcludePackagesProxyEndpointIP: "10.241.32.22",
			RetryAttempts:                  1,
			RetryInterval:                  time.Millisecond,
		},
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "excluded IP '10.241.32.22' is still present")
	require.ErrorContains(t, err, "Current endpoints:")
	require.ErrorContains(t, err, "Current endpoints: 10.241.32.22:4219,10.241.36.14:4219")
}

func TestGetCloudConfigWithOptions_ReturnsBase64CloudConfigWhenIPExcluded(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	cloudConfig := `#!/bin/bash
export PACKAGES_PROXY_ADDRESSES="10.241.36.14:4219"`
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manual-bootstrap-for-master",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: map[string][]byte{
			"cloud-config":           []byte(cloudConfig),
			"packagesProxyAddresses": []byte("- 10.241.36.14:4219\n"),
		},
	}

	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(context.Background(), secret, metav1.CreateOptions{})
	require.NoError(t, err)

	got, err := GetCloudConfigWithOptions(
		context.Background(),
		kubeCl,
		"master",
		false,
		log.GetDefaultLogger(),
		CloudConfigOptions{
			ExcludePackagesProxyEndpointIP: "10.241.32.22",
			RetryAttempts:                  1,
			RetryInterval:                  time.Millisecond,
		},
	)
	require.NoError(t, err)

	want := base64.StdEncoding.EncodeToString([]byte(cloudConfig))
	require.Equal(t, want, got)
}

func TestGetCloudConfigWithOptions_UsesStructuredPackagesProxyAddresses(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	cloudConfig := `#!/bin/bash
export PACKAGES_PROXY_ADDRESSES="10.241.32.22:4219,10.241.36.14:4219"`
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manual-bootstrap-for-master",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: map[string][]byte{
			"cloud-config":           []byte(cloudConfig),
			"packagesProxyAddresses": []byte("- 10.241.36.14:4219\n"),
		},
	}

	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(context.Background(), secret, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = GetCloudConfigWithOptions(
		context.Background(),
		kubeCl,
		"master",
		false,
		log.GetDefaultLogger(),
		CloudConfigOptions{
			ExcludePackagesProxyEndpointIP: "10.241.32.22",
			RetryAttempts:                  1,
			RetryInterval:                  time.Millisecond,
		},
	)
	require.NoError(t, err)
}

func TestResolveCloudConfigRetryOptions(t *testing.T) {
	tests := []struct {
		name             string
		opts             CloudConfigOptions
		expectedAttempts int
		expectedInterval time.Duration
	}{
		{
			name:             "defaults for zero values",
			opts:             CloudConfigOptions{},
			expectedAttempts: defaultCloudConfigRetryAttempts,
			expectedInterval: defaultCloudConfigRetryInterval,
		},
		{
			name: "custom positive values",
			opts: CloudConfigOptions{
				RetryAttempts: 7,
				RetryInterval: 3 * time.Second,
			},
			expectedAttempts: 7,
			expectedInterval: 3 * time.Second,
		},
		{
			name: "defaults for negative values",
			opts: CloudConfigOptions{
				RetryAttempts: -1,
				RetryInterval: -1 * time.Second,
			},
			expectedAttempts: defaultCloudConfigRetryAttempts,
			expectedInterval: defaultCloudConfigRetryInterval,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attempts, interval := resolveCloudConfigRetryOptions(tc.opts)
			require.Equal(t, tc.expectedAttempts, attempts)
			require.Equal(t, tc.expectedInterval, interval)
		})
	}
}
