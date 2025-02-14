package k8s

import (
	"k8s.io/client-go/kubernetes"

	config "node-proxy-sidecar/internal/config"
	"node-proxy-sidecar/internal/haproxy"
)

type Endpoint struct {
	Name      string
	Addresses []string
	Ports     []int32
}

type Client struct {
	client *kubernetes.Clientset
}

type BackendUpdate struct {
	Backend config.Backend
	Servers []haproxy.Server
}
