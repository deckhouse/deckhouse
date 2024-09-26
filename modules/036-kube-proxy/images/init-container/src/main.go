/*
Copyright 2022 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

const (
	apiProxyAddress             = "127.0.0.1:6445"
	bindInternalIPAnnotationKey = "node.deckhouse.io/nodeport-bind-internal-ip"

	kubeConfigPath      = "/var/lib/kube-proxy/kubeconfig.conf"
	kubeProxyConfigPath = "/var/lib/kube-proxy/config.conf"
)

func main() {
	podSubnet, ok := os.LookupEnv("POD_SUBNET")
	if !ok {
		log.Fatal("POD_SUBNET env not provided")
	}
	controlPlaneAddress, ok := os.LookupEnv("CONTROL_PLANE_ADDRESS")
	if !ok {
		log.Fatal("CONTROL_PLANE_ADDRESS env not provided")
	}

	apiProxyAddress, err := getApiProxyAddress()
	if err != nil {
		log.Fatal(err)
	}
	if len(apiProxyAddress) != 0 {
		controlPlaneAddress = apiProxyAddress
	}

	nodePortBindInternalIP, err := getNodePortBindInternalIP(controlPlaneAddress)
	if err != nil {
		log.Fatal(err)
	}

	kubeConfig := &configv1.Config{
		Kind:       "Config",
		APIVersion: configv1.SchemeGroupVersion.String(),
		Clusters: []configv1.NamedCluster{
			{
				Name: "default",
				Cluster: configv1.Cluster{
					Server:               "https://" + controlPlaneAddress,
					CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name: "default",
				AuthInfo: configv1.AuthInfo{
					TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: "default",
				Context: configv1.Context{
					Cluster:   "default",
					Namespace: "default",
					AuthInfo:  "default",
				},
			},
		},
		CurrentContext: "default",
	}

	featureGates := map[string]bool{
		"TopologyAwareHints": true,
	}

	kubernetesVersion, err := semver.NewVersion(os.Getenv("KUBERNETES_VERSION"))
	if err != nil {
		log.Fatalf("Error parsing kubernetes version: %s", err)
	}

	// The ProxyTerminatingEndpoints feature gate has been removed in Kubernetes v1.30.
	k8s130, _ := semver.NewVersion("1.30")
	if kubernetesVersion.LessThan(k8s130) {
		featureGates["ProxyTerminatingEndpoints"] = true
	}

	// The DaemonSetUpdateSurge feature gate has been removed in Kubernetes v1.27.
	k8s127, _ := semver.NewVersion("1.27")
	if kubernetesVersion.LessThan(k8s127) {
		featureGates["DaemonSetUpdateSurge"] = true
	}

	// The EndpointSliceTerminatingCondition feature gate has been removed in Kubernetes v1.28.
	k8s128, _ := semver.NewVersion("1.28")
	if kubernetesVersion.LessThan(k8s128) {
		featureGates["EndpointSliceTerminatingCondition"] = true
	}

	kubeProxyConfig := &v1alpha1.KubeProxyConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeProxyConfiguration",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		FeatureGates: featureGates,
		ClusterCIDR:  podSubnet,
		ClientConnection: alpha1.ClientConnectionConfiguration{
			Kubeconfig: "/var/lib/kube-proxy/kubeconfig.conf",
		},
		Mode: "iptables",
		Conntrack: v1alpha1.KubeProxyConntrackConfiguration{
			MaxPerCore: pointer.Int32(0),
		},
		NodePortAddresses: []string{nodePortBindInternalIP},
	}

	kubeConfigBytes, err := yaml.Marshal(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}
	kubeProxyConfigBytes, err := yaml.Marshal(kubeProxyConfig)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(kubeConfigPath, kubeConfigBytes, 0644)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(kubeProxyConfigPath, kubeProxyConfigBytes, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func getNodePortBindInternalIP(apiAddress string) (string, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	inClusterConfig.Host = "https://" + apiAddress

	clientSet, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		return "", err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get pod hostname: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	node, err := clientSet.CoreV1().Nodes().Get(ctx, hostname, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if os.Getenv("CLOUD_PROVIDER") == "gcp" {
		return "0.0.0.0/0", nil
	}

	v, ok := node.GetAnnotations()[bindInternalIPAnnotationKey]
	if ok && v == "false" {
		return "0.0.0.0/0", nil
	}

	var firstInternalAddress string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			firstInternalAddress = addr.Address
			break
		}
	}

	if len(firstInternalAddress) == 0 {
		return "", fmt.Errorf("failed to found InternalIP for Node %s", node.GetName())
	}

	return firstInternalAddress + "/32", nil
}

func getApiProxyAddress() (string, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	inClusterConfig.Host = "https://" + apiProxyAddress
	inClusterConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{}

	restClient, err := rest.UnversionedRESTClientFor(inClusterConfig)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := restClient.Get().AbsPath("/api").Do(ctx).Error(); err != nil {
		log.Printf("Failed to contact apiserver through %s: %s", apiProxyAddress, err)
		return "", nil
	}

	return apiProxyAddress, nil
}
