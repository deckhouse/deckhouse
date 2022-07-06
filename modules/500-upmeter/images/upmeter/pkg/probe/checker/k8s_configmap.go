/*
Copyright 2021 Flant JSC

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// KubeObjectBasicLifecycle checks the creation and deletion of an object in
// kube-apiserver. Hence, all errors in kube-apiserver calls result in probe fails.
type KubeObjectBasicLifecycle struct {
	preflight doer
	creator   doer
	getter    doer
	deleter   doer
}

func (c *KubeObjectBasicLifecycle) Check() check.Error {
	ctx := context.TODO()
	if err := c.preflight.Do(ctx); err != nil {
		return check.ErrUnknown("preflight: %v", err)
	}

	// Check garbage
	if getErr := c.getter.Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		// Unexpected error
		return check.ErrFail("getting garbage: %v", getErr)
	} else if getErr == nil {
		// Garbage object exists, cleaning it and skipping this run.
		if delErr := c.deleter.Do(ctx); delErr != nil {
			return check.ErrFail("deleting garbage: %v", delErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	// The actual check
	if createErr := c.creator.Do(ctx); createErr != nil {
		// Unexpected error
		return check.ErrFail("creating: %v", createErr)
	}
	if delErr := c.deleter.Do(ctx); delErr != nil {
		// Unexpected error
		return check.ErrFail("deleting: %v", delErr)
	}

	return nil
}

// ConfigMapLifecycle4 is a checker constructor and configurator
type ConfigMapLifecycle4 struct {
	Access    kubernetes.Access
	Timeout   time.Duration
	Namespace string
}

func (c ConfigMapLifecycle4) Checker() check.Checker {
	preflight := newK8sVersionGetter(c.Access)

	name := run.StaticIdentifier("upmeter-probe-basic")
	creator := &configmapCreator{access: c.Access, namespace: c.Namespace, name: name}
	getter := &configmapGetter{access: c.Access, namespace: c.Namespace, name: name}
	deleter := &configmapDeleter{access: c.Access, namespace: c.Namespace, name: name}

	checker := &KubeObjectBasicLifecycle{
		preflight: preflight,
		creator:   creator,
		getter:    getter,
		deleter:   deleter,
	}

	return withTimeout(checker, c.Timeout)
}

type configmapCreator struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *configmapCreator) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	cm := createConfigMapObject(c.name)
	_, err := client.CoreV1().ConfigMaps(c.namespace).Create(cm)
	return err
}

type configmapGetter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *configmapGetter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().ConfigMaps(c.namespace).Get(c.name, metav1.GetOptions{})
	return err
}

type configmapDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *configmapDeleter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().ConfigMaps(c.namespace).Delete(c.name, &metav1.DeleteOptions{})
	return err
}

func createConfigMapObject(name string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				"upmeter-agent": run.ID(),
				"upmeter-group": "control-plane",
				"upmeter-probe": "basic",
			},
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
}
