// Copyright 2025 Flant JSC
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

package hooks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

type PodRef struct {
	Name      string
	Namespace string
	Ready     bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/upmeter/adjust_retention",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "adjust_retention_every_15min",
			Crontab: "0 */4 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "upmeter_pod",
			ApiVersion:             "v1",
			Kind:                   "Pod",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-upmeter"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "upmeter",
				},
			},
			FilterFunc: filterUpmeterPod,
		},
	},
}, dependency.WithExternalDependencies(adjustUpmeterRetention))

func filterUpmeterPod(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to Pod: %v", err)
	}

	isReady := false
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == "upmeter" {
			isReady = cs.Ready
			break
		}
	}

	return PodRef{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Ready:     isReady,
	}, nil
}

func adjustUpmeterRetention(input *go_hook.HookInput, dc dependency.Container) error {
	snap := input.Snapshots["upmeter_pod"]
	if len(snap) == 0 {
		return nil
	}

	pod := snap[0].(PodRef)
	if !pod.Ready {
		return fmt.Errorf("upmeter pod is not ready")
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("unable to get Kubernetes client: %w", err)
	}

	podObj, err := kubeClient.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get pod object: %w", err)
	}

	var currentRetention int
	for _, container := range podObj.Spec.Containers {
		if container.Name == "upmeter" {
			for _, envVar := range container.Env {
				if envVar.Name == "RETENTION_DAYS" {
					currentRetention, err = strconv.Atoi(envVar.Value)
					if err != nil {
						return fmt.Errorf("invalid RETENTION_DAYS env value: %v", err)
					}
					break
				}
			}
		}
	}

	stdout, _, err := execToPod(kubeClient, "df -B1 --output=pcent /db", "upmeter", pod.Name, pod.Namespace)
	if err != nil {
		return fmt.Errorf("unable to execute command in pod: %w", err)
	}

	usagePercent, err := parseDFPct(stdout)
	if err != nil {
		return fmt.Errorf("unable to parse disk usage percentage: %w", err)
	}

	if usagePercent > 50 {
		currentRetention -= 7
		if currentRetention < 90 {
			currentRetention = 90
		}
	}

	input.Values.Set("upmeter.internal.retentionDays", currentRetention)

	return nil
}

func parseDFPct(output string) (int, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "%") {
			pctStr := strings.TrimSuffix(line, "%")
			pct, err := strconv.Atoi(strings.TrimSpace(pctStr))
			if err != nil {
				return 0, fmt.Errorf("failed to convert to int: %w", err)
			}
			return pct, nil
		}
	}
	return 0, fmt.Errorf("no usage percentage found in df output")
}

func execToPod(kubeClient k8s.Client, command, container, podName, namespace string) (string, string, error) {
	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Timeout(10 * time.Second)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	paramCodec := runtime.NewParameterCodec(scheme)

	req.VersionedParams(&corev1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: container,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, paramCodec)

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", "", err
	}

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout strings.Builder
	var stderr strings.Builder

	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}
