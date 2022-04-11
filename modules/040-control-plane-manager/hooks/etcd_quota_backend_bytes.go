/*
Copyright 2022 Flant JSC

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

package hooks

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type etcdNode struct {
	memory      int64
	isDedicated bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        etcdMaintenanceQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/master": "",
				},
			},
			FilterFunc: etcdQuotaFilterNode,
		},
		etcdMaintenanceConfig,
		etcdSecretK8sConfig,
	},
}, etcdQuotaBackendBytesHandler)

func etcdQuotaFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	memory := node.Status.Capacity.Memory().Value()

	isDedicated := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/master" && taint.Effect == corev1.TaintEffectNoSchedule {
			isDedicated = true
			break
		}
	}

	return &etcdNode{
		memory:      memory,
		isDedicated: isDedicated,
	}, nil
}

func etcdQuotaBackendBytesHandler(input *go_hook.HookInput) error {

	for _, endpointRaw := range input.Snapshots[etcdEndpointsSnapshotName] {
		instance := endpointRaw.(*etcdInstance)
		status, err := etcdClient.Status(context.TODO(), instance.Endpoint)
		if err != nil {
			input.LogEntry.Errorf("cannot get current db usage from %s: %v", err, instance.PodName)
			continue
		}

		if float64(status.DbSize)/float64(instance.MaxDbSize) < 0.9 {
			input.LogEntry.Debugf("Etcd instance '%s' does not need to defrag", instance.PodName)
			continue
		}

		input.LogEntry.Warnf("Start defrag etcd instance '%s' %d/%d", instance.PodName, status.DbSize, instance.MaxDbSize)
		_, err = etcdClient.Defragment(context.TODO(), instance.Endpoint)
		if err != nil {
			input.MetricsCollector.Inc("etcd_defragmentation_failed_total", map[string]string{
				"pod_name": instance.PodName,
				"node":     instance.Node,
			})
			input.LogEntry.Errorf("Defrag etcd '%s' instance finished with err: %v", instance.PodName, err)
			continue
		}

		input.MetricsCollector.Inc("etcd_defragmentation_success_total", map[string]string{
			"pod_name": instance.PodName,
			"node":     instance.Node,
		})

		input.LogEntry.Infof("Defrag etcd '%s' instanse finished successfully", instance.PodName)
	}

	return nil
}
