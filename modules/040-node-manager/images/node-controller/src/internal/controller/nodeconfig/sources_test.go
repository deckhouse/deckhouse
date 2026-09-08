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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	sigsyaml "sigs.k8s.io/yaml"
)

func TestPickKubeletDigest(t *testing.T) {
	tests := []struct {
		name     string
		packages map[string]string
		version  string
		want     string
	}{
		{
			name: "two-digit patch wins over one-digit (numeric, not lexicographic)",
			packages: map[string]string{
				"kubeletSysext1356":  "sha256:patch6",
				"kubeletSysext13510": "sha256:patch10",
			},
			version: "1.35",
			want:    "sha256:patch10", // 1.35.10 > 1.35.6
		},
		{
			name:     "another minor version is not this one's kubelet",
			packages: map[string]string{"kubeletSysext1346": "sha256:v134"},
			version:  "1.35",
			want:     "",
		},
		{
			name:     "no matching prefix yields empty",
			packages: map[string]string{"other": "x"},
			version:  "1.35",
			want:     "",
		},
		{
			name:     "non-numeric suffix is ignored",
			packages: map[string]string{"kubeletSysext135abc": "x", "kubeletSysext1355": "sha256:v5"},
			version:  "1.35",
			want:     "sha256:v5",
		},
		{
			name:     "no version derived yields empty rather than any kubelet at all",
			packages: map[string]string{"kubeletSysext1356": "sha256:patch6"},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickKubeletDigest(tt.packages, tt.version); got != tt.want {
				t.Fatalf("pickKubeletDigest = %q, want %q", got, tt.want)
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
		require.Equal(t, "k8s.internal", config.ClusterDomain)
	})

	// ClusterConfiguration writes the prefix as a string; a rendered copy of it
	// can carry a bare number. Both have to mean the same thing.
	t.Run("the pod subnet prefix, quoted or bare", func(t *testing.T) {
		quoted := sourceReaderOver(dnsCluster(t, clusterConfigSecret("podSubnetNodeCIDRPrefix: \"22\"\n")))
		config, err := quoted.readClusterConfiguration(t.Context())
		require.NoError(t, err)
		require.Equal(t, 500, defaultMaxPodsFor(config.PodSubnetNodeCIDRPrefix))

		bare := sourceReaderOver(dnsCluster(t, clusterConfigSecret("podSubnetNodeCIDRPrefix: 23\n")))
		config, err = bare.readClusterConfiguration(t.Context())
		require.NoError(t, err)
		require.Equal(t, 250, defaultMaxPodsFor(config.PodSubnetNodeCIDRPrefix))
	})
}

// The cluster-wide half of the render inputs, end to end: a configuration that
// names no domain gets the one ClusterConfiguration itself defaults to, and a
// missing object stops the pass instead of handing every node a made-up value.
func TestReadClusterState(t *testing.T) {
	objects := []client.Object{
		apiServerEndpointSlice("10.0.0.1"),
		dnsService(kubeDNSServiceName, "kube-dns", "10.0.0.10"),
		clusterCAConfigMapObject("-----BEGIN CERTIFICATE-----"),
	}

	t.Run("a configuration that names no domain keeps the default", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, append(objects, clusterConfigSecret("kubernetesVersion: \"1.35\"\n"))...))

		in := clusterInputs{}
		require.NoError(t, s.readClusterState(t.Context(), &in))

		require.Equal(t, defaultClusterDomain, in.ClusterDomain)
		require.Equal(t, []string{"https://10.0.0.1:6443"}, in.APIServerEndpoints)
		require.Equal(t, "10.0.0.10", in.ClusterDNS)
		require.NotEmpty(t, in.KubernetesCA)
	})

	t.Run("the configured domain wins", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, append(objects, clusterConfigSecret("clusterDomain: k8s.internal\n"))...))

		in := clusterInputs{}
		require.NoError(t, s.readClusterState(t.Context(), &in))

		require.Equal(t, "k8s.internal", in.ClusterDomain)
	})

	t.Run("a cluster with no CA to hand out is not rendered at all", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t,
			apiServerEndpointSlice("10.0.0.1"),
			dnsService(kubeDNSServiceName, "kube-dns", "10.0.0.10"),
			clusterConfigSecret("clusterDomain: k8s.internal\n"),
		))

		in := clusterInputs{}
		require.ErrorContains(t, s.readClusterState(t.Context(), &in), clusterCAConfigMap)
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
		{name: "a /24 slice per node", prefix: intstr.FromString("24"), expMaxPods: 120},
		{name: "a /23 slice per node", prefix: intstr.FromString("23"), expMaxPods: 250},
		{name: "a /22 slice per node", prefix: intstr.FromString("22"), expMaxPods: 500},
		{name: "a slice narrower than /24", prefix: intstr.FromString("25"), expMaxPods: 120},
		{
			// A bashible node advertises 1000 here. An immutable node beside it
			// advertising 500 is the scheduler skew this ladder exists to avoid,
			// so the whole ladder has to fit under the agent's schema.
			name:       "a /21 slice per node, as wide as bashible goes",
			prefix:     intstr.FromString("21"),
			expMaxPods: 1000,
		},
		{name: "no prefix configured falls back to a /24", expMaxPods: 120},
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

// A cluster carries the static configuration or the provider one, never both,
// so neither absence may stop a pass: a cloud cluster would go unrendered for
// lacking a secret only a static cluster has.
func TestReadInternalNetworkCIDRs(t *testing.T) {
	t.Run("a cluster that named no networks at all", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t))

		cidrs, err := s.readInternalNetworkCIDRs(t.Context())

		require.NoError(t, err)
		require.Empty(t, cidrs)
	})

	t.Run("the networks a static cluster was configured with", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, staticConfigSecret(
			"kind: StaticClusterConfiguration\ninternalNetworkCIDRs:\n- 192.168.42.0/24\n- 172.16.16.0/24\n")))

		cidrs, err := s.readInternalNetworkCIDRs(t.Context())

		require.NoError(t, err)
		require.Equal(t, []string{"192.168.42.0/24", "172.16.16.0/24"}, cidrs)
	})

	t.Run("a static configuration that names none", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, staticConfigSecret("kind: StaticClusterConfiguration\n")))

		cidrs, err := s.readInternalNetworkCIDRs(t.Context())

		require.NoError(t, err)
		require.Empty(t, cidrs)
	})

	// Not an empty answer: an unreadable configuration rendered as "no networks"
	// would walk that change through every node of the fleet.
	t.Run("a static configuration that cannot be parsed", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, staticConfigSecret("\tnot: [yaml")))

		_, err := s.readInternalNetworkCIDRs(t.Context())

		require.ErrorContains(t, err, staticConfigSecretName)
	})

	t.Run("the subnet a cloud cluster addresses its nodes in", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t, providerConfigSecret(
			"kind: YandexClusterConfiguration\nnodeNetworkCIDR: 10.241.32.0/20\n")))

		cidrs, err := s.readInternalNetworkCIDRs(t.Context())

		require.NoError(t, err)
		require.Equal(t, []string{"10.241.32.0/20"}, cidrs)
	})

	// A hybrid cluster has both. The static list comes first, since it is the
	// one an operator wrote by hand.
	t.Run("both configurations", func(t *testing.T) {
		s := sourceReaderOver(dnsCluster(t,
			staticConfigSecret("internalNetworkCIDRs:\n- 192.168.42.0/24\n"),
			providerConfigSecret("kind: AzureClusterConfiguration\nsubnetCIDR: 10.50.0.0/16\n"),
		))

		cidrs, err := s.readInternalNetworkCIDRs(t.Context())

		require.NoError(t, err)
		require.Equal(t, []string{"192.168.42.0/24", "10.50.0.0/16"}, cidrs)
	})
}

// The provider table copies ten schemas
// (modules/030-cloud-provider-*/candi/openapi/cluster_configuration.yaml, and
// the ee ones); a wrong path is a node told nothing and left to guess.
func TestProviderInternalNetworkCIDR(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "OpenStack Standard",
			config: "kind: OpenStackClusterConfiguration\nlayout: Standard\nstandard:\n  internalNetworkCIDR: 192.168.199.0/24\n",
			want:   "192.168.199.0/24",
		},
		{
			name:   "OpenStack StandardWithNoRouter",
			config: "kind: OpenStackClusterConfiguration\nlayout: StandardWithNoRouter\nstandardWithNoRouter:\n  internalNetworkCIDR: 192.168.198.0/24\n",
			want:   "192.168.198.0/24",
		},
		{
			// The layout names an existing subnet instead, so the cluster has no
			// CIDR to hand out and the node picks its address itself.
			name:   "OpenStack SimpleWithInternalNetwork, which names a subnet and no CIDR",
			config: "kind: OpenStackClusterConfiguration\nlayout: SimpleWithInternalNetwork\nsimpleWithInternalNetwork:\n  internalSubnetName: kube\n",
		},
		{
			name:   "HuaweiCloud Standard",
			config: "kind: HuaweiCloudClusterConfiguration\nlayout: Standard\nstandard:\n  internalNetworkCIDR: 192.168.199.0/24\n",
			want:   "192.168.199.0/24",
		},
		{
			name:   "HuaweiCloud VpcPeering",
			config: "kind: HuaweiCloudClusterConfiguration\nlayout: VpcPeering\nvpcPeering:\n  internalNetworkCIDR: 192.168.198.0/24\n",
			want:   "192.168.198.0/24",
		},
		{
			name:   "vSphere",
			config: "kind: VsphereClusterConfiguration\ninternalNetworkCIDR: 192.168.199.0/24\n",
			want:   "192.168.199.0/24",
		},
		{
			name:   "VCD",
			config: "kind: VCDClusterConfiguration\nlayout: WithNAT\ninternalNetworkCIDR: 192.168.199.0/24\n",
			want:   "192.168.199.0/24",
		},
		{
			name:   "Yandex",
			config: "kind: YandexClusterConfiguration\nnodeNetworkCIDR: 10.241.32.0/20\n",
			want:   "10.241.32.0/20",
		},
		{
			name:   "Dynamix StandardWithInternalNetwork",
			config: "kind: DynamixClusterConfiguration\nlayout: StandardWithInternalNetwork\nnodeNetworkCIDR: 10.241.32.0/20\n",
			want:   "10.241.32.0/20",
		},
		{
			name:   "AWS",
			config: "kind: AWSClusterConfiguration\nnodeNetworkCIDR: 10.241.32.0/20\n",
			want:   "10.241.32.0/20",
		},
		{
			// Optional in every AWS layout: left out, the nodes take the whole
			// VPC range, which nothing here can work out.
			name:   "AWS without the optional subnet",
			config: "kind: AWSClusterConfiguration\nvpcNetworkCIDR: 10.241.0.0/16\n",
		},
		{
			name:   "Azure",
			config: "kind: AzureClusterConfiguration\nsubnetCIDR: 10.50.0.0/16\n",
			want:   "10.50.0.0/16",
		},
		{
			name:   "GCP",
			config: "kind: GCPClusterConfiguration\nsubnetworkCIDR: 10.0.0.0/24\n",
			want:   "10.0.0.0/24",
		},
		{
			// Neither schema has such a field: the node is addressed by the
			// virtualization layer, and there is nothing to publish.
			name:   "DVP",
			config: "kind: DVPClusterConfiguration\nlayout: Standard\n",
		},
		{
			name:   "zVirt",
			config: "kind: ZvirtClusterConfiguration\nlayout: Standard\n",
		},
		{
			name:   "a provider this table does not know",
			config: "kind: SomeFutureClusterConfiguration\nnodeNetworkCIDR: 10.0.0.0/24\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config map[string]any
			require.NoError(t, sigsyaml.Unmarshal([]byte(tt.config), &config))

			cidr, err := providerInternalNetworkCIDR(config)

			require.NoError(t, err)
			require.Equal(t, tt.want, cidr)
		})
	}

	// A configuration whose field is not a string is a broken document, and
	// rendering stops rather than quietly telling the fleet nothing.
	t.Run("a subnet that is not a string", func(t *testing.T) {
		var config map[string]any
		require.NoError(t, sigsyaml.Unmarshal([]byte("kind: YandexClusterConfiguration\nnodeNetworkCIDR: 10\n"), &config))

		_, err := providerInternalNetworkCIDR(config)

		require.ErrorContains(t, err, "nodeNetworkCIDR")
	})
}

func staticConfigSecret(config string) client.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: staticConfigSecretName},
		Data:       map[string][]byte{staticConfigKey: []byte(config)},
	}
}

func providerConfigSecret(config string) client.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: providerConfigSecretName},
		Data:       map[string][]byte{providerConfigKey: []byte(config)},
	}
}

func sourceReaderOver(cluster client.Client) *sourceReader {
	return &sourceReader{Reader: cluster}
}

func apiServerEndpointSlice(address string) client.Object {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: apiServerEndpointSliceNS, Name: apiServerEndpointSliceName},
		Ports: []discoveryv1.EndpointPort{{
			Name: ptr.To(apiServerPortName),
			Port: ptr.To(int32(apiserverPort)),
		}},
		Endpoints: []discoveryv1.Endpoint{{Addresses: []string{address}}},
	}
}

func clusterCAConfigMapObject(ca string) client.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterCAConfigMap},
		Data:       map[string]string{clusterCAKey: ca},
	}
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

// The agent image carries no version in its name — it is built from a pinned
// commit of the agent repository's main — so its digest is looked up by exact
// key rather than through soleDigest, which needs a numeric tail. A release
// that did not build it is a build defect: silently dropping the extension
// would leave every node's agent frozen at whatever the OS image carries.
func TestSysextDigestsAgent(t *testing.T) {
	packages := map[string]string{
		"containerdSysext224":    "sha256:c",
		"kubernetesCniSysext162": "sha256:n",
		"kubeletSysext1356":      "sha256:k",
		"nodeletSysext":          "sha256:a",
	}

	t.Run("the agent digest is picked up", func(t *testing.T) {
		got, err := sysextDigests(map[string]map[string]string{registryPackagesDigestsKey: packages}, "1.35")
		require.NoError(t, err)
		require.Equal(t, "sha256:a", got[nodeletExtension])
	})

	t.Run("a release without the agent image is refused", func(t *testing.T) {
		without := make(map[string]string, len(packages))
		for name, digest := range packages {
			if name == "nodeletSysext" {
				continue
			}
			without[name] = digest
		}

		_, err := sysextDigests(map[string]map[string]string{registryPackagesDigestsKey: without}, "1.35")
		require.ErrorContains(t, err, nodeletExtension)
	})
}
