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
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// StatefulSetPodLifecycle is a checker constructor and configurator
type StatefulSetPodLifecycle struct {
	Access kubernetes.Access

	Namespace string
	AgentID   string

	CreationTimeout      time.Duration
	DeletionTimeout      time.Duration
	PodTransitionTimeout time.Duration
}

func (c StatefulSetPodLifecycle) Checker() check.Checker {
	preflight := newK8sVersionGetter(c.Access)

	stsName := run.StaticIdentifier("upmeter-probe-controller-manager")
	podName := stsName + "-0"
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

	stsPodPresenceGetter := &pollingDoer{
		doer:     &podGetter{access: c.Access, namespace: c.Namespace, name: podName},
		catch:    func(err error) bool { return err == nil },
		timeout:  c.PodTransitionTimeout,
		interval: c.PodTransitionTimeout / 10,
	}

	stsPodAbsenceGetter := &pollingDoer{
		doer:     &podGetter{access: c.Access, namespace: c.Namespace, name: podName},
		catch:    func(err error) bool { return apierrors.IsNotFound(err) },
		timeout:  c.PodTransitionTimeout,
		interval: c.PodTransitionTimeout / 10,
	}

	stsPodDeleter := &podDeleter{access: c.Access, namespace: c.Namespace, name: podName}

	checker := &KubeControllerObjectLifecycle{
		preflight: preflight,

		parentGetter:  stsGetter,
		parentCreator: stsCreator,
		parentDeleter: stsDeleter,

		childPresenceGetter: stsPodPresenceGetter,
		childAbsenceGetter:  stsPodAbsenceGetter,
		childDeleter:        stsPodDeleter,
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
	_, err := client.AppsV1().StatefulSets(s.namespace).Get(s.name, metav1.GetOptions{})
	return err
}

type statefulSetCreator struct {
	access    kubernetes.Access
	namespace string
	sts       *appsv1.StatefulSet
}

func (s statefulSetCreator) Do(ctx context.Context) error {
	client := s.access.Kubernetes()
	_, err := client.AppsV1().StatefulSets(s.namespace).Create(s.sts)
	return err
}

type statefulSetDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (s statefulSetDeleter) Do(ctx context.Context) error {
	client := s.access.Kubernetes()
	err := client.AppsV1().StatefulSets(s.namespace).Delete(s.name, &metav1.DeleteOptions{})
	return err
}

type pollingDoer struct {
	doer     doer
	catch    func(error) bool
	timeout  time.Duration
	interval time.Duration
}

func (p *pollingDoer) Do(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	deadline := time.NewTimer(p.timeout)

	defer ticker.Stop()
	defer deadline.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.doer.Do(ctx); err != nil && !apierrors.IsNotFound(err) {
				return err // arbitrary error
			} else if p.catch(err) {
				return err // desired state
			}
		case <-deadline.C:
			return fmt.Errorf("polling timeout reached")
		}
	}
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
