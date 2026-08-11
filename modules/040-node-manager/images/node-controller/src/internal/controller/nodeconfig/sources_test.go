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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewestPatchDigest(t *testing.T) {
	tests := []struct {
		name     string
		packages map[string]string
		prefix   string
		want     string
	}{
		{
			name: "two-digit patch wins over one-digit (numeric, not lexicographic)",
			packages: map[string]string{
				"kubeletSysext1356":  "sha256:patch6",
				"kubeletSysext13510": "sha256:patch10",
			},
			prefix: "kubeletSysext135",
			want:   "sha256:patch10", // 1.35.10 > 1.35.6
		},
		{
			name:     "no matching prefix yields empty",
			packages: map[string]string{"other": "x"},
			prefix:   "kubeletSysext",
			want:     "",
		},
		{
			name:     "non-numeric suffix is ignored",
			packages: map[string]string{"kubeletSysextabc": "x", "kubeletSysext5": "sha256:v5"},
			prefix:   "kubeletSysext",
			want:     "sha256:v5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newestPatchDigest(tt.packages, tt.prefix); got != tt.want {
				t.Fatalf("newestPatchDigest = %q, want %q", got, tt.want)
			}
		})
	}
}

// The camelcase image name strips version separators, so no "newest" can be
// told: several candidates are a build defect to report, not an ordering
// problem to solve. The same rule lives in dhctl's soleDigest.
func TestSoleDigest(t *testing.T) {
	t.Run("exactly one is returned", func(t *testing.T) {
		d, err := soleDigest(map[string]string{"containerdSysext224": "sha256:v224"}, "containerdSysext")
		require.NoError(t, err)
		require.Equal(t, "sha256:v224", d)
	})
	t.Run("none is empty, not an error", func(t *testing.T) {
		d, err := soleDigest(map[string]string{"other": "x"}, "containerdSysext")
		require.NoError(t, err)
		require.Empty(t, d)
	})
	t.Run("a non-numeric tail is a different image", func(t *testing.T) {
		d, err := soleDigest(map[string]string{
			"containerdSysext224":     "sha256:v224",
			"containerdSysextDebug":   "x",
			"containerdSysextLegacy2": "x",
		}, "containerdSysext")
		require.NoError(t, err)
		require.Equal(t, "sha256:v224", d)
	})
	t.Run("several candidates are refused with both names", func(t *testing.T) {
		_, err := soleDigest(map[string]string{
			"containerdSysext224":  "sha256:v224",
			"containerdSysext2210": "sha256:v2210",
		}, "containerdSysext")
		require.Error(t, err)
		require.Contains(t, err.Error(), "containerdSysext224")
		require.Contains(t, err.Error(), "containerdSysext2210")
	})
}

// The cluster configuration decides the node's DNS domain and how many pods it
// advertises. Reading it used to degrade silently: a failure yielded the
// defaults, and the whole group was reconfigured onto them.
func TestReadClusterConfiguration(t *testing.T) {
	t.Run("no cluster configuration to read", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t))

		_, err := s.readClusterConfiguration(t.Context())

		require.ErrorContains(t, err, clusterConfigSecretName)
	})

	t.Run("a cluster configuration that cannot be parsed", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, clusterConfigSecret("\tnot: [yaml")))

		_, err := s.readClusterConfiguration(t.Context())

		require.ErrorContains(t, err, clusterConfigSecretName)
	})

	t.Run("the domain the cluster was configured with", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, clusterConfigSecret("clusterDomain: k8s.internal\n")))

		config, err := s.readClusterConfiguration(t.Context())

		require.NoError(t, err)
		require.Equal(t, "k8s.internal", config.clusterDomain())
	})

	t.Run("a configuration that names no domain keeps the default", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, clusterConfigSecret("kubernetesVersion: \"1.35\"\n")))

		config, err := s.readClusterConfiguration(t.Context())

		require.NoError(t, err)
		require.Equal(t, defaultClusterDomain, config.clusterDomain())
	})

	// ClusterConfiguration writes the prefix as a string; a rendered copy of it
	// can carry a bare number. Both have to mean the same thing.
	t.Run("the pod subnet prefix, quoted or bare", func(t *testing.T) {
		quoted := sourceReaderOver(dnsCluster(t, clusterConfigSecret("podSubnetNodeCIDRPrefix: \"22\"\n")))
		config, err := quoted.readClusterConfiguration(t.Context())
		require.NoError(t, err)
		require.Equal(t, maxPodsPerNodeCIDR22, defaultMaxPodsFor(config.PodSubnetNodeCIDRPrefix))

		bare := sourceReaderOver(dnsCluster(t, clusterConfigSecret("podSubnetNodeCIDRPrefix: 23\n")))
		config, err = bare.readClusterConfiguration(t.Context())
		require.NoError(t, err)
		require.Equal(t, maxPodsPerNodeCIDR23, defaultMaxPodsFor(config.PodSubnetNodeCIDRPrefix))
	})
}

// A node's pod ceiling follows its slice of the pod subnet, the way bashible's
// does: a flat 120 next to bashible's 500 on a /22 cluster is the scheduler
// skew this number exists to avoid.
func TestDefaultMaxPodsFor(t *testing.T) {
	tests := []struct {
		name       string
		prefix     intstr.IntOrString
		expMaxPods int
	}{
		{name: "a /24 slice per node", prefix: intstr.FromString("24"), expMaxPods: maxPodsPerNodeCIDR24},
		{name: "a /23 slice per node", prefix: intstr.FromString("23"), expMaxPods: maxPodsPerNodeCIDR23},
		{name: "a /22 slice per node", prefix: intstr.FromString("22"), expMaxPods: maxPodsPerNodeCIDR22},
		{name: "a slice narrower than /24", prefix: intstr.FromString("25"), expMaxPods: maxPodsPerNodeCIDR24},
		{
			// Bashible allows 1000 here; an immutable node's schema stops at 500,
			// so it gets the ceiling rather than a config the API server refuses.
			name:       "a /21 slice per node is capped at what the node accepts",
			prefix:     intstr.FromString("21"),
			expMaxPods: maxPodsCeiling,
		},
		{name: "no prefix configured falls back to a /24", expMaxPods: maxPodsPerNodeCIDR24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expMaxPods, defaultMaxPodsFor(tt.prefix))
		})
	}
}

// With several DNS-labelled services and no kube-dns, the winner must not
// depend on listing order (the address changed between passes). Finding none is
// an error — an empty address would roll the group onto a DNS-less config.
func TestReadClusterDNS(t *testing.T) {
	t.Run("the same service every time", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t,
			dnsService("coredns-b", "coredns", "10.0.0.22"),
			dnsService("coredns-a", "coredns", "10.0.0.11"),
		))

		for range 3 {
			dns, err := s.readClusterDNS(t.Context())

			require.NoError(t, err)
			require.Equal(t, "10.0.0.11", dns)
		}
	})

	t.Run("kube-dns wins outright", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t,
			dnsService("coredns-a", "coredns", "10.0.0.11"),
			dnsService(kubeDNSServiceName, "kube-dns", "10.0.0.10"),
		))

		dns, err := s.readClusterDNS(t.Context())

		require.NoError(t, err)
		require.Equal(t, "10.0.0.10", dns)
	})

	t.Run("no DNS service at all is an error, not an empty address", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, dnsService("other", "something-else", "10.0.0.33")))

		_, err := s.readClusterDNS(t.Context())

		require.ErrorContains(t, err, "no DNS service")
	})

	t.Run("a headless DNS service is no address either", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, dnsService(kubeDNSServiceName, "kube-dns", corev1.ClusterIPNone)))

		_, err := s.readClusterDNS(t.Context())

		require.ErrorContains(t, err, "no DNS service")
	})
}

// A docker config the node cannot be given credentials from is an error: read as
// "this registry is anonymous", it hands every node of the group a pull that
// fails against a private registry with nothing saying why.
func TestRegistryAuth(t *testing.T) {
	auth, err := registryAuth([]byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`), "registry.example.com")
	require.NoError(t, err)
	require.Equal(t, "dXNlcjpwYXNz", auth)

	// An anonymous registry genuinely has no credentials.
	auth, err = registryAuth(nil, "registry.example.com")
	require.NoError(t, err)
	require.Empty(t, auth)

	_, err = registryAuth([]byte("{not json"), "registry.example.com")
	require.ErrorContains(t, err, registryDockerConfigKey)
}

// sourceReaderOver reads a test cluster through both readers. The two are
// separate on purpose in production — cached and live — and a unit test that
// left one of them nil would only find out by panicking.
func sourceReaderOver(cluster client.Client) *sourceReader {
	return &sourceReader{Client: cluster, Reader: cluster}
}

func dnsCluster(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	coreOnly := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(coreOnly))
	return fake.NewClientBuilder().WithScheme(coreOnly).WithObjects(objects...).Build()
}

func clusterConfigSecret(config string) client.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterConfigSecretName},
		Data:       map[string][]byte{clusterConfigKey: []byte(config)},
	}
}

func dnsService(name, app, clusterIP string) client.Object {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      name,
			Labels:    map[string]string{dnsAppLabel: app},
		},
		Spec: corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}
