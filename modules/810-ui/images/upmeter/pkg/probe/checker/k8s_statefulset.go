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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// StatefulSetPodLifecycle is a checker constructor and configurator
type StatefulSetPodLifecycle struct {
	Access    kubernetes.Access
	Preflight Doer
	Namespace string

	AgentID string
	Name    string

	CreationTimeout      time.Duration
	DeletionTimeout      time.Duration
	PodTransitionTimeout time.Duration
}

func (c StatefulSetPodLifecycle) Checker() check.Checker {
	stsName, podName := c.Name, c.Name+"-0"
	sts := createStatefulSetObject(stsName, c.AgentID)

	stsGetter := &statefulSetGetter{access: c.Access, namespace: c.Namespace, name: stsName}

	stsCreator := doWithTimeout(
		&statefulSetCreator{access: c.Access, namespace: c.Namespace, sts: sts},
		c.CreationTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	stsDeleter := doWithTimeout(
		&statefulSetDeleter{access: c.Access, namespace: c.Namespace, name: stsName},
		c.DeletionTimeout,
		fmt.Errorf("deletion timeout reached"),
	)

	stsPodGetter := &podGetter{access: c.Access, namespace: c.Namespace, name: podName}
	stsPodDeleter := &podDeleter{access: c.Access, namespace: c.Namespace, name: podName}

	// Not to rarely
	pollInterval := c.PodTransitionTimeout / 10
	if pollInterval > 5*time.Second {
		pollInterval = 5 * time.Second
	}

	checker := &KubeControllerObjectLifecycle{
		preflight: c.Preflight,

		parentGetter:  stsGetter,
		parentCreator: stsCreator,
		parentDeleter: stsDeleter,

		childGetter:          stsPodGetter,
		childDeleter:         stsPodDeleter,
		childPollingInterval: pollInterval,
		childPollingTimeout:  c.PodTransitionTimeout,
	}

	return checker
}

type statefulSetGetter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (s statefulSetGetter) Do(ctx context.Context) error {
	client := s.access.Kubernetes()
	_, err := client.AppsV1().StatefulSets(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	return err
}

type statefulSetCreator struct {
	access    kubernetes.Access
	namespace string
	sts       *appsv1.StatefulSet
}

func (s statefulSetCreator) Do(ctx context.Context) error {
	client := s.access.Kubernetes()
	_, err := client.AppsV1().StatefulSets(s.namespace).Create(ctx, s.sts, metav1.CreateOptions{})
	return err
}

type statefulSetDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (s statefulSetDeleter) Do(ctx context.Context) error {
	client := s.access.Kubernetes()
	err := client.AppsV1().StatefulSets(s.namespace).Delete(ctx, s.name, metav1.DeleteOptions{})
	return err
}

func createStatefulSetObject(name, agentID string) *appsv1.StatefulSet {
	replicas := int32(1)

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": "control-plane",
				"upmeter-probe": "controller-manager",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					agentLabelKey:   agentID,
					"upmeter-group": "control-plane",
					"upmeter-probe": "controller-manager",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"heritage":      "upmeter",
						agentLabelKey:   agentID,
						"upmeter-group": "control-plane",
						"upmeter-probe": "controller-manager",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "pause",
							Image: "registry.k8s.io/upmeter-nonexistent:3.1415",
							Command: []string{
								"/pause",
							},
						},
					},
					NodeSelector: map[string]string{
						"label-to-avoid":          "scheduling-this-pod-on-any-node",
						"upmeter-only-tests-that": "controller-manager-creates-pods",
					},
					Tolerations: []v1.Toleration{
						{Operator: v1.TolerationOpExists},
					},
				},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}
}
