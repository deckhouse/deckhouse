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

package kubeclient

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"fencing-agent/internal/domain"
)

func NewRestConfig() (*rest.Config, error) {
	return config.GetConfig()
}

func New(cfg *rest.Config) (kubernetes.Interface, error) {
	// Core-group requests use protobuf: the bootstrap retry relists the whole
	// NodeGroup, and protobuf decodes several times cheaper than JSON at fleet
	// scale. CRD clients must keep JSON, so mutate a copy.
	cfg = rest.CopyConfig(cfg)
	cfg.ContentType = runtime.ContentTypeProtobuf

	return kubernetes.NewForConfig(cfg)
}

func ResolveIdentity(ctx context.Context, k8s kubernetes.Interface, nodeName string) (domain.NodeIdentity, error) {
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return domain.NodeIdentity{}, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	// The InternalIP becomes the memberlist advertise address, so an empty one
	// silently degrades into advertising the pod IP, which peers cannot reach.
	ip := internalIP(node)
	if ip == "" {
		return domain.NodeIdentity{}, fmt.Errorf("node %q has no InternalIP address", nodeName)
	}

	return domain.NodeIdentity{
		Name: node.Name,
		UID:  string(node.UID),
		IP:   ip,
	}, nil
}
