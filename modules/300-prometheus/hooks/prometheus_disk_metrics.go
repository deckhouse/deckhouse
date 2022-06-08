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

package hooks

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/prometheus_disk_metrics",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "main",
			Crontab: "*/10 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "prometheus",
				},
			},
			FilterFunc: applyPodFilter,
		},
	},
}, dependency.WithExternalDependencies(prometheusDiskMetrics))

type PodFilter struct {
	Name                     string
	Namespace                string
	PrometheusContainerReady bool
}

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod = &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	containerReady := false
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "prometheus" {
			containerReady = status.Ready
			break
		}
	}

	return PodFilter{
		Name:                     pod.Name,
		Namespace:                pod.Namespace,
		PrometheusContainerReady: containerReady,
	}, nil
}

func prometheusDiskMetrics(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	pods := input.Snapshots["pods"]
	for _, obj := range pods {
		pod := obj.(PodFilter)

		if !pod.PrometheusContainerReady {
			continue
		}

		fsSize, fsUsed, fsUsePercent := getFsInfo(input, kubeClient, pod)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_size",
			fsSize,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_used",
			fsUsed,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_use_percent",
			fsUsePercent,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)
	}
	return nil
}

func getFsInfo(input *go_hook.HookInput, kubeClient k8s.Client, pod PodFilter) (fsSize, fsUsed, fsUsePercent float64) {
	containerName := "prometheus"
	command := "df -PB1 /prometheus/"
	output, _, err := execToPodThroughAPI(kubeClient, command, containerName, pod.Name, pod.Namespace)
	if err != nil {
		input.LogEntry.Warnf("%s: %s", pod.Name, err.Error())
	} else {
		for _, s := range strings.Split(output, "\n") {
			if strings.Contains(s, "prometheus") {
				fsSize, _ = strconv.ParseFloat(strings.Fields(s)[1],64)
				fsUsed, _ = strconv.ParseFloat(strings.Fields(s)[2],64)
				fsUsePercent, _ = strconv.ParseFloat(strings.Trim(strings.Fields(s)[4], "%"),64)
				break
			}
		}
	}
	return
}

func execToPodThroughAPI(kubeClient k8s.Client, command, containerName, podName, namespace string) (string, string, error) {
	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	config, err := rest.InClusterConfig()
	if err != nil {
		return "", "", err
	}

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}
