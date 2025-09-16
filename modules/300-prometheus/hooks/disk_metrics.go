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
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/disk_metrics",
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
					"app.kubernetes.io/name": "prometheus",
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

func prometheusDiskMetrics(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	for pod, err := range sdkobjectpatch.SnapshotIter[PodFilter](input.Snapshots.Get("pods")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'pods' snapshot: %v", err)
		}

		if !pod.PrometheusContainerReady {
			continue
		}

		fsSizeBytes, fsUsedBytes, fsUsedPercent := getFsInfo(input, kubeClient, pod)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_size_bytes",
			fsSizeBytes,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_used_bytes",
			fsUsedBytes,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)

		input.MetricsCollector.Set(
			"d8_prometheus_fs_used_percent",
			fsUsedPercent,
			map[string]string{
				"namespace": pod.Namespace,
				"pod_name":  pod.Name,
			},
			metrics.WithGroup("prometheus_disk_hook"),
		)
	}
	return nil
}

func getFsInfo(input *go_hook.HookInput, kubeClient k8s.Client, pod PodFilter) (float64, float64, float64) {
	var (
		command                                 = "df -PB1 /prometheus/"
		containerName                           = "prometheus"
		fsSizeBytes, fsUsedBytes, fsUsedPercent float64
	)

	output, _, err := execToPodThroughAPI(kubeClient, command, containerName, pod.Name, pod.Namespace)
	if err != nil {
		input.Logger.Warn("exec to pod through api", slog.String("pod_name", pod.Name), log.Err(err))
	} else {
		for _, s := range strings.Split(output, "\n") {
			if strings.Contains(s, "prometheus") {
				fsSizeBytes, _ = strconv.ParseFloat(strings.Fields(s)[1], 64)
				fsUsedBytes, _ = strconv.ParseFloat(strings.Fields(s)[2], 64)
				fsUsedPercent, _ = strconv.ParseFloat(strings.Trim(strings.Fields(s)[4], "%"), 64)
				break
			}
		}
	}

	return fsSizeBytes, fsUsedBytes, fsUsedPercent
}

func execToPodThroughAPI(kubeClient k8s.Client, command, containerName, podName, namespace string) (string, string, error) {
	req := kubeClient.CoreV1().RESTClient().Post().
		Timeout(time.Duration(10) * time.Second).
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
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
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
