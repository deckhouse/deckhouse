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

// Package kubeclient constructs Kubernetes clients and resolves the local
// node identity. Follow-up tasks extend it with the seed list lookup for
// memberlist and the watch of the agent's own Node (maintenance annotations,
// planned removal).
package kubeclient

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"fencing-agent/internal/domain"
)

func NewRestConfig() (*rest.Config, error) {
	return config.GetConfig()
}

func New(cfg *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(cfg)
}

func ResolveIdentity(ctx context.Context, k8s kubernetes.Interface, nodeName string) (domain.NodeIdentity, error) {
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return domain.NodeIdentity{}, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	identity := domain.NodeIdentity{
		Name: node.Name,
		UID:  string(node.UID),
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			identity.IP = addr.Address
			break
		}
	}

	return identity, nil
}
