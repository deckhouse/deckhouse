package k8s

import "k8s.io/client-go/kubernetes"

type Endpoint struct {
	Name      string
	Addresses []string
	Ports     []int32
}

type Client struct {
	client *kubernetes.Clientset
}
