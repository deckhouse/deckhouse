/*
Copyright 2023 Flant JSC

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

package checker

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// ConfigMapLifecycle is a checker constructor and configurator
type ConfigMapLifecycle struct {
	Access    kubernetes.Access
	Preflight Doer
	Namespace string

	AgentID string
	Name    string

	Timeout time.Duration
}

func (c ConfigMapLifecycle) Checker() check.Checker {
	cm := createConfigMapObject(c.Name, c.AgentID)
	creator := &configmapCreator{access: c.Access, namespace: c.Namespace, cm: cm}
	getter := &configmapGetter{access: c.Access, namespace: c.Namespace, name: c.Name}
	deleter := &configmapDeleter{access: c.Access, namespace: c.Namespace, name: c.Name}

	checker := &KubeObjectBasicLifecycle{
		preflight: c.Preflight,
		creator:   creator,
		getter:    getter,
		deleter:   deleter,
	}

	return withTimeout(checker, c.Timeout)
}

type configmapCreator struct {
	access    kubernetes.Access
	namespace string
	cm        *v1.ConfigMap
}

func (c *configmapCreator) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().ConfigMaps(c.namespace).Create(ctx, c.cm, metav1.CreateOptions{})
	return err
}

type configmapGetter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *configmapGetter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	return err
}

type configmapDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *configmapDeleter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().ConfigMaps(c.namespace).Delete(ctx, c.name, metav1.DeleteOptions{})
	return err
}

const agentLabelKey = "upmeter-agent"

func createConfigMapObject(name, agentID string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": "control-plane",
				"upmeter-probe": "basic",
			},
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
}
