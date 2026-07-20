// Copyright 2021 Flant JSC
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

package deckhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func cleanupDeckhousePods(ctx context.Context, kubeCl *client.KubernetesClient, pods *v1.PodList) *v1.PodList {
	d8Pods := &v1.PodList{}

	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodSucceeded, v1.PodFailed, v1.PodUnknown:
			if err := kubeCl.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
				dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Failed to delete pod %s. err: %v", pod.Name, err), "\n"))
			} else {
				dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Pod %s was successfully deleted", pod.Name), "\n"))
			}
		default:
			d8Pods.Items = append(d8Pods.Items, pod)
		}
	}
	return d8Pods
}

func GetPod(ctx context.Context, kubeCl *client.KubernetesClient, leaderElectionLeaseName types.NamespacedName) (*v1.Pod, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pods, err := kubeCl.CoreV1().Pods("d8-system").List(ctx, metav1.ListOptions{LabelSelector: "app=deckhouse"})
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Cannot get deckhouse pod. Got error: %v", err), "\n"))
		return nil, ErrListPods
	}
	pods = cleanupDeckhousePods(ctx, kubeCl, pods)

	if len(pods.Items) == 0 {
		dhlog.FromContext(ctx).DebugContext(ctx, "Cannot get deckhouse pod. Count of returned pods is zero")
		return nil, ErrListPods
	}

	if len(pods.Items) == 1 {
		return new(pods.Items[0]), nil
	}

	return getLeaderElectionLeaseHolderPod(ctx, kubeCl, leaderElectionLeaseName, pods)
}

func getLeaderElectionLeaseHolderPod(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	leaderElectionLeaseName types.NamespacedName,
	pods *v1.PodList,
) (*v1.Pod, error) {
	lease, err := kubeCl.
		CoordinationV1().
		Leases(leaderElectionLeaseName.Namespace).
		Get(ctx, leaderElectionLeaseName.Name, metav1.GetOptions{})
	switch {
	case err != nil:
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Cannot get deckhouse pod. Got error reading lease: %v", err), "\n"))
		return nil, ErrReadLease
	case lease.Spec.HolderIdentity == nil:
		dhlog.FromContext(ctx).DebugContext(ctx, "No Deckhouse leader election lease holder identity found")
		return nil, ErrBadLease
	case lease.Spec.RenewTime == nil:
		dhlog.FromContext(ctx).DebugContext(ctx, "No Deckhouse leader election lease renew time found")
		return nil, ErrBadLease
	case lease.Spec.LeaseDurationSeconds == nil:
		dhlog.FromContext(ctx).DebugContext(ctx, "No Deckhouse leader election lease duration seconds found")
		return nil, ErrBadLease
	}

	leaseRenewTime := *lease.Spec.RenewTime
	leaseDuration := time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	if time.Since(leaseRenewTime.Time) >= leaseDuration {
		dhlog.FromContext(ctx).DebugContext(ctx, "Deckhouse leader election lease is expired")
		return nil, ErrBadLease
	}

	for _, pod := range pods.Items {
		if pod.Name == strings.Split(*lease.Spec.HolderIdentity, ".")[0] {
			return new(pod), nil
		}
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "Pod specified as Deckhouse leader election lease holder does not exist")
	return nil, ErrListPods
}
