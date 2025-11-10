/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checks

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"safe-updater/internal/constant"
)

type checkResult string

const (
	Allowed checkResult = "allowed"
	Denied  checkResult = "denied"
	Abort   checkResult = "abort"
)

type ExternalCheck interface {
	GetCheckResult(*corev1.Pod) checkResult
}

type cniCiliumCheck struct {
	podsByNodes map[string][]*corev1.Pod
	daemonSet   *appsv1.DaemonSet
}

func NewCniCiliumCheck(ctx context.Context, klient client.Client) (*cniCiliumCheck, error) {
	pods := new(corev1.PodList)
	if err := klient.List(ctx, pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app":    "agent",
			"module": "cni-cilium",
		}),
		Namespace: constant.CiliumNamespace,
	}); err != nil {
		return nil, err
	}

	podsByNodeNames := make(map[string][]*corev1.Pod, len(pods.Items))
	for _, pod := range pods.Items {
		podsByNodeName, found := podsByNodeNames[pod.Spec.NodeName]
		if !found {
			podsByNodeName = make([]*corev1.Pod, 0, 1)
		}
		podsByNodeName = append(podsByNodeName, &pod)
		podsByNodeNames[pod.Spec.NodeName] = podsByNodeName
	}

	daemonSet := new(appsv1.DaemonSet)
	if err := klient.Get(ctx, client.ObjectKey{Name: constant.CiliumDaemonSet, Namespace: constant.CiliumNamespace}, daemonSet); err != nil {
		return nil, err
	}

	return &cniCiliumCheck{
		podsByNodes: podsByNodeNames,
		daemonSet:   daemonSet,
	}, nil
}

func (c *cniCiliumCheck) GetCheckResult(pod *corev1.Pod) checkResult {
	if !DaemonSetIsUpToDate(c.daemonSet) {
		return Abort
	}

	pods := c.podsByNodes[pod.Spec.NodeName]

	if len(pods) != 1 {
		klog.Warningf("there are %d cilium pods on the %s node", len(pods), pod.Spec.NodeName)
		return Denied
	}

	ciliumPod := pods[0]

	if !PodIsReadyAndRunning(ciliumPod) {
		klog.Warningf("the %s cilium pod on the %s node is not up and running", ciliumPod.Name, pod.Spec.NodeName)
		return Denied
	}

	if !ciliumPod.DeletionTimestamp.IsZero() {
		klog.Warningf("the %s cilium pod on the %s node is terminating", ciliumPod.Name, pod.Spec.NodeName)
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
