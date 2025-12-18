/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checks

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"safe-updater/internal/constant"
)

type checkResult string

const (
	Allowed checkResult = "allowed"
	Denied  checkResult = "denied"
)

type ExternalCheck interface {
	GetCheckResult(*corev1.Pod) checkResult
	GetName() string
}

type cniCiliumCheck struct {
	podsByNodes map[string][]*corev1.Pod
	dsRevision  string
}

func NewCniCiliumCheck(ctx context.Context, pods *corev1.PodList, currentDsRevision string) (*cniCiliumCheck, error) {
	podsByNodeNames := make(map[string][]*corev1.Pod, len(pods.Items))
	for _, pod := range pods.Items {
		podsByNodeName, found := podsByNodeNames[pod.Spec.NodeName]
		if !found {
			podsByNodeName = make([]*corev1.Pod, 0, 1)
		}
		podsByNodeName = append(podsByNodeName, &pod)
		podsByNodeNames[pod.Spec.NodeName] = podsByNodeName
	}

	return &cniCiliumCheck{
		podsByNodes: podsByNodeNames,
		dsRevision:  currentDsRevision,
	}, nil
}

func (c *cniCiliumCheck) GetName() string {
	return "cilium"
}

func (c *cniCiliumCheck) GetCheckResult(pod *corev1.Pod) checkResult {
	pods := c.podsByNodes[pod.Spec.NodeName]

	if len(pods) != 1 {
		klog.Warningf("updating %s: there are %d cilium pods on the %s node", pod.Name, len(pods), pod.Spec.NodeName)
		return Denied
	}

	ciliumPod := pods[0]

	if !PodIsReadyAndRunning(ciliumPod) {
		klog.Warningf("updating %s: the %s cilium pod on the %s node is not up and running", pod.Name, ciliumPod.Name, pod.Spec.NodeName)
		return Denied
	}

	if !ciliumPod.DeletionTimestamp.IsZero() {
		klog.Warningf("updating %s: the %s cilium pod on the %s node is terminating", pod.Name, ciliumPod.Name, pod.Spec.NodeName)
		return Denied
	}

	if ciliumPod.GetLabels()[constant.PodTemplateGenerationLabel] != c.dsRevision {
		klog.Warningf("updating %s: the %s cilium pod on the %s node has wrong template generation", pod.Name, ciliumPod.Name, pod.Spec.NodeName)
		return Denied
	}

	return Allowed
}

func PodIsReadyAndRunning(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}

func DaemonSetIsUpToDate(ds *appsv1.DaemonSet) bool {
	return ds.GetGeneration() == ds.Status.ObservedGeneration &&
		ds.Status.DesiredNumberScheduled == ds.Status.UpdatedNumberScheduled
}

func DaemonSetIsStable(pods *corev1.PodList) bool {
	for _, pod := range pods.Items {
		if !PodIsReadyAndRunning(&pod) || !pod.DeletionTimestamp.IsZero() {
			return false
		}
	}

	return true
}
