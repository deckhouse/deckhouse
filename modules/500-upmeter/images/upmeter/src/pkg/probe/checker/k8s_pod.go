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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

const svcName = "deckhouse-leader"

// AtLeastOnePodReady is a checker constructor and configurator
type AtLeastOnePodReady struct {
	Access        kubernetes.Access
	Namespace     string
	LabelSelector string

	Timeout time.Duration

	// PreflightChecker verifies preconditions before running the check
	PreflightChecker check.Checker
}

func (c AtLeastOnePodReady) Checker() check.Checker {
	podsChecker := &podReadinessChecker{
		access:        c.Access,
		namespace:     c.Namespace,
		labelSelector: c.LabelSelector,
		client:        newInsecureClient(3 * c.Timeout),
	}

	return sequence(
		c.PreflightChecker,
		withTimeout(podsChecker, c.Timeout),
	)
}

// podReadinessChecker defines the information that lets check at least one ready pod
type podReadinessChecker struct {
	namespace     string
	labelSelector string
	access        kubernetes.Access
	client        *http.Client
}

func (c *podReadinessChecker) Check() check.Error {
	podList, err := c.access.Kubernetes().CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: c.labelSelector})
	if err != nil {
		return check.ErrUnknown("cannot get pods %s,%s: %v", c.namespace, c.labelSelector, err)
	}

	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			return nil
		}
	}

	return check.ErrFail("no ready pods found %s,%s", c.namespace, c.labelSelector)
}

// podRunningOrReadyChecker checks that there is a pod in Ready condition in reasonable time.
//   - if pod is running, but not ready in the `readinessTimeout` time, the check status is considered unknown.
//   - if pod is running, but not ready in `readinessTimeout` or more, the check status is considered failed.
//   - if pod is terminating, the status is unknown
//   - otherwise, the status is success.
type podRunningOrReadyChecker struct {
	namespace        string
	labelSelector    string
	readinessTimeout time.Duration
	access           kubernetes.Access
	client           http.Client
}

type Status struct {
	ConvergeInProgress        *int `json:"CONVERGE_IN_PROGRESS"`
	ConvergeWaitTask          bool `json:"CONVERGE_WAIT_TASK"`
	StartupConvergeDone       bool `json:"STARTUP_CONVERGE_DONE"`
	StartupConvergeInProgress *int `json:"STARTUP_CONVERGE_IN_PROGRESS"`
	StartupConvergeNotStarted bool `json:"STARTUP_CONVERGE_NOT_STARTED"`
}

const (
	windowSize          = 6
	taskGrowthThreshold = 0.01
	freezeThreshold     = 5 * time.Minute
)

var history []Status

func (c *podRunningOrReadyChecker) poll() (*Status, check.Error) {
	url, err := c.extractServiceURL()
	if err != nil {
		return nil, check.ErrUnknown("cannot get svc url %s: %v", c.namespace, err)
	}

	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, url, nil)
	if err != nil {
		return nil, check.ErrUnknown("error getting deckhouse pod status converge %s: %v", c.namespace, err)
	}

	resp, err := c.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, check.ErrUnknown("failed to get status converge %s: %v", c.namespace, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, check.ErrUnknown("failed to get status converge %s: %v", c.namespace, err)
	}

	var status Status
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, check.ErrUnknown("failed unmarshal json %s: %v", c.namespace, err)
	}

	return &status, nil
}

func (c *podRunningOrReadyChecker) Check() check.Error {
	status, err := c.poll()
	if err != nil {
		return err
	}
	fmt.Printf("poll ended %+v\n", status)
	history = append(history, *status)
	if len(history) > windowSize {
		println("history more than window size, removing oldest element")
		history = history[1:]
	}
	fmt.Printf("append to history %+v\n", history)

	if len(history) < 2 {
		// not enough data
		fmt.Printf("not enough data yet, %d\n", len(history))
		return nil
	}

	latest := history[len(history)-1]

	if latest.ConvergeWaitTask {
		print("queue is empty, skip check")

		// queue is empty, deckhouse is waiting for tasks
		return nil
	}

	start, end := history[0], latest
	startTasks := toInt(start.ConvergeInProgress) + toInt(start.StartupConvergeInProgress)
	endTasks := toInt(end.ConvergeInProgress) + toInt(end.StartupConvergeInProgress)
	fmt.Printf("start tasks %d, end tasks %d\n", startTasks, endTasks)

	duration := time.Duration(len(history)) * (time.Second * 60)
	fmt.Printf("duration %d\n", duration)

	growthRate := float64(endTasks-startTasks) / duration.Seconds()
	fmt.Printf("growth rate %d\n", growthRate)

	if growthRate > taskGrowthThreshold {
		fmt.Printf("growth rate exceeds task growth threshold %d\n", growthRate)
		return check.ErrFail("growth rate exceeds task growth threshold")
	}

	// check for frozen queue
	allEqual := true
	ref := toInt(history[0].ConvergeInProgress) + toInt(history[0].StartupConvergeInProgress)
	for _, h := range history[1:] {
		cur := toInt(h.ConvergeInProgress) + toInt(h.StartupConvergeInProgress)
		if cur != ref {
			allEqual = false
			break
		}
	}

	if allEqual {
		frozenDuration := time.Duration(len(history)) * (time.Second * 60)
		if frozenDuration >= freezeThreshold {
			return check.ErrFail("queue size haven't changed in 5 minutes, possibly frozen")
		}
	}

	return nil
}

func toInt(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func isPodRunning(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning
}

func isPodPending(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodPending
}

func isPodTerminating(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func isPodReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != v1.PodReady {
			// not the condition type we are looking for
			continue
		}
		return cond.Status == v1.ConditionTrue
	}

	return false
}

func isPodWorking(pod *v1.Pod, readinessTimeout time.Duration) check.Error {
	if pod.DeletionTimestamp != nil {
		return check.ErrUnknown("pod is terminating")
	}

	if isPodReady(pod) {
		return nil
	}

	if !isPodRunning(pod) {
		return check.ErrFail("pod is not running")
	}

	// Let's see how long it is running
	readinessThreshold := pod.CreationTimestamp.Add(readinessTimeout)
	if time.Now().After(readinessThreshold) {
		return check.ErrFail("pod is not ready for too long (%s)", readinessTimeout.String())
	}

	return check.ErrUnknown("pod is running, but not ready")
}

func createPodObject(podName, nodeName, agentID string, image *kubernetes.ProbeImage) *v1.Pod {
	nodeAffinity := createNodeAffinityObject(nodeName)

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": "control-plane",
				"upmeter-probe": "scheduler",
			},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: image.PullSecrets(),
			Containers: []v1.Container{
				{
					Name:            "pause",
					Image:           image.Name(),
					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"true",
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations: []v1.Toleration{
				{Operator: v1.TolerationOpExists},
			},
			Affinity: &v1.Affinity{
				NodeAffinity: nodeAffinity,
			},
		},
	}
}

func createNodeAffinityObject(nodeName string) *v1.NodeAffinity {
	return &v1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{
				{
					MatchExpressions: []v1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/hostname",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{nodeName},
						},
					},
				},
			},
		},
	}
}

func (c *podRunningOrReadyChecker) extractServiceURL() (string, error) {
	service, err := c.access.Kubernetes().CoreV1().Services(c.namespace).Get(context.TODO(), svcName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var serviceport int32
	for _, port := range service.Spec.Ports {
		if port.Name == "self" {
			serviceport = port.Port
		}
	}

	return fmt.Sprintf("http://%s.%s:%d/status/converge?output=json", svcName, c.namespace, serviceport), nil
}
