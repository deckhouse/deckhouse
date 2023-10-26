// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ns    = "d8-system"
	name  = "docs-sync"
	label = "deckhouse.io/documentation-builder-sync"
)

func NewConfigmapManager() (*ConfigmapManager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("create rest config: %w", err)
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("new client set: %w", err)
	}

	return &ConfigmapManager{kclient: kclient}, nil
}

type ConfigmapManager struct {
	name string

	kclient *kubernetes.Clientset
}

func (m *ConfigmapManager) Create(ctx context.Context) error {
	cm := &core.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Labels:       map[string]string{label: ""},
		},
		Data: map[string]string{
			"address": fmt.Sprintf(
				"%s.%s.pod.cluster.local",
				strings.ReplaceAll(os.Getenv(""), ".", "-"), //TODO: container ip
				os.Getenv(""), // TODO: namespace name
			),
		},
	}

	cm, err := m.kclient.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	m.name = cm.Name
	return nil
}

func (m *ConfigmapManager) Remove() error {
	if m.name == "" {
		return nil
	}

	err := m.kclient.CoreV1().ConfigMaps(ns).Delete(context.Background(), m.name, metav1.DeleteOptions{})
	m.name = ""
	return err
}
