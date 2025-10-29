/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultUserAgent = "d8-shutdown-inhibitor"
	defaultQPS       = 5
	defaultBurst     = 10
	defaultTimeout   = 15 * time.Second
)

type Klient struct {
	clientset kubeclient.Interface
}

func NewClientFromKubeconfig(path string) (*Klient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, fmt.Errorf("build config from kubeconfig %q: %w", path, err)
	}

	return newClient(cfg)
}

func NewClientFromServiceAccount() (*Klient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("build in-cluster config: %w", err)
	}
	return newClient(cfg)
}

func newClient(cfg *rest.Config) (*Klient, error) {
	cfg = rest.CopyConfig(cfg)
	cfg.UserAgent = defaultUserAgent
	cfg.QPS = defaultQPS
	cfg.Burst = defaultBurst
	cfg.Timeout = defaultTimeout
	clientset, err := kubeclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %w", err)
	}

	return &Klient{clientset: clientset}, nil
}

func (c *Klient) Clientset() kubeclient.Interface {
	return c.clientset
}

// ListPodsOnNode returns pods scheduled onto the provided node using a field selector.
func (c *Klient) ListPodsOnNode(ctx context.Context, nodeName string) (*PodList, error) {
	sel := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pl, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: sel})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %q: %w", nodeName, err)
	}
	return pl, nil
}
