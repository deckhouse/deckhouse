package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
		log.Fatal("CONTROL_PLANE_ADDRESS not provided")
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
		APIVersion: "v1",
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

	kubeProxyConfig := &v1alpha1.KubeProxyConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeProxyConfiguration",
			APIVersion: "kubeproxy.config.k8s.io/v1alpha1",
		},
		FeatureGates: map[string]bool{
			"EndpointSliceTerminatingCondition": true,
			"ProxyTerminatingEndpoints":         true,
			"DaemonSetUpdateSurge":              true,
			"TopologyAwareHints":                true,
		},
		ClusterCIDR: podSubnet,
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

	err = os.WriteFile(kubeConfigPath, kubeConfigBytes, 0666)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(kubeProxyConfigPath, kubeProxyConfigBytes, 0666)
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

	if v, ok := node.GetAnnotations()[bindInternalIPAnnotationKey]; !ok || v == "false" || os.Getenv("CLOUD_PROVIDER") == "gcp" {
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

	return firstInternalAddress, nil
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
