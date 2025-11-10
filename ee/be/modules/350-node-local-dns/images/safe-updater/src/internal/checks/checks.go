/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checks

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"safe-updater/internal/constant"
)

type CheckResult string

const (
	Allowed = "allowed"
	Denied  = "denied"
	Abort   = "abort"
)

type ExternalCheck interface {
	GetCheckResult(*corev1.Pod) bool
}

type CniCiliumCheck struct {
	pods      *corev1.PodList
	daemonSet *appsv1.DaemonSet
}

func NewCniCiliumCheck(ctx context.Context, klient client.Client, nodeName string) (*CniCiliumCheck, error) {
	pods := new(corev1.PodList)
	if err := klient.List(ctx, pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app":    "agent",
			"module": "cni-cilium",
		}),
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"spec.nodeName": nodeName,
		}),
		Namespace: constant.CiliumNamespace,
	}); err != nil {
		return nil, err
	}

	daemonSet := new(appsv1.DaemonSet)

	return &CniCiliumCheck{
		pods:      pods,
		daemonSet: daemonSet,
	}, nil
}

func (c *CniCiliumCheck) GetCheckResult(pod *corev1.Pod) CheckResult {
	if !DaemonSetIsUpToDate(c.daemonSet) {
		return Abort
	}

	if len(c.pods.Items) != 1 {
		klog.Warningf("there are %d cilium pods on the %s node", len(c.pods.Items), pod.Spec.NodeName)
		return Denied
	}

	ciliumPod := c.pods.Items[0]

	if !PodIsReadyAndRunning(&ciliumPod) {
		klog.Warningf("the %s cilium pod on the %s node is not up and running", ciliumPod.Name, pod.Spec.NodeName)
		return Denied
	}

	if !pod.DeletionTimestamp.IsZero() {
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
