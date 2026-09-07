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

package bashiblecontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dnsService(name, clusterIP, app string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      name,
			Labels:    map[string]string{dnsAppLabel: app},
		},
		Spec: corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}

func TestReadGlobals_AllSources(t *testing.T) {
	clusterConfig := `apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
podSubnetNodeCIDRPrefix: "24"
clusterDomain: cluster.local
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
proxy:
  httpProxy: http://p
  noProxy:
    - example.com
`
	s := newService(t,
		configMap(versionInfoCMNS, versionInfoCMName, map[string]string{
			"data.json": `{"channel":"stable","version":"v1.2.3","edition":"EE"}`,
		}),
		secret(kubeSystemNS, clusterConfigSecretName, map[string][]byte{
			clusterConfigKey: []byte(clusterConfig),
		}),
		configMap(kubeSystemNS, clusterUUIDConfigMapName, map[string]string{
			clusterUUIDKey: "uuid-1",
		}),
		dnsService("d8-kube-dns", "10.222.0.10", "kube-dns"),
	)

	g := s.ReadGlobals(context.Background())
	assert.Equal(t, "stable", g.DeckhouseChannel)
	assert.Equal(t, "v1.2.3", g.DeckhouseVersion)
	assert.Equal(t, "EE", g.DeckhouseEdition)
	assert.Equal(t, "24", g.PodSubnetNodeCIDRPrefix)
	assert.Equal(t, "cluster.local", g.ClusterDomain)
	assert.Equal(t, "uuid-1", g.ClusterUUID)
	assert.Equal(t, "10.222.0.10", g.ClusterDNSAddress)
	assert.Equal(t, map[string]interface{}{
		"httpProxy": "http://p",
		"noProxy": []interface{}{
			"127.0.0.1", "169.254.169.254", "cluster.local",
			"10.111.0.0/16", "10.222.0.0/16", "example.com",
		},
	}, g.Proxy)
}

func TestReadGlobals_EmptyCluster(t *testing.T) {
	g := newService(t).ReadGlobals(context.Background())
	assert.Equal(t, Globals{}, g)
	assert.Nil(t, g.Proxy)
}

func TestReadClusterDNSAddress_PrefersKubeDNS(t *testing.T) {
	s := newService(t,
		dnsService("d8-kube-dns", "10.222.0.20", "coredns"),
		dnsService("kube-dns", "10.222.0.10", "kube-dns"),
		dnsService("kube-dns-headless", "None", "kube-dns"),
	)
	assert.Equal(t, "10.222.0.10", s.readClusterDNSAddress(context.Background()))
}

func TestReadClusterConfiguration_NoProxyKeyOmitsBlock(t *testing.T) {
	s := newService(t, secret(kubeSystemNS, clusterConfigSecretName, map[string][]byte{
		clusterConfigKey: []byte("clusterDomain: cluster.local\npodSubnetNodeCIDRPrefix: \"24\"\n"),
	}))
	cfg := s.readClusterConfiguration(context.Background())
	assert.NotNil(t, cfg)
	assert.Nil(t, buildProxy(cfg))
}
