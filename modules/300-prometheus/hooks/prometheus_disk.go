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
	"errors"
	"fmt"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ceph-csi",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "main",
			Crontab: "*/15 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "scs",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: applyStorageclassFilter,
		},
		{
			Name:       "pvcs",
			ApiVersion: "v1",
			Kind:       "PersistentVolumeClaim",
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
			FilterFunc: applyPersistentVolumeClaimFilter,
		},
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
		{
			Name:       "proms",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "Prometheus",
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
			FilterFunc: applyPromFilter,
		},
	},
}, dependency.WithExternalDependencies(prometheusDisk))

type StorageClassFilter struct {
	Name                 string
	AllowVolumeExpansion bool
}

func applyStorageclassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc = &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return StorageClassFilter{
		Name:                 sc.Name,
		AllowVolumeExpansion: *sc.AllowVolumeExpansion && sc.Annotations["storageclass.deckhouse.io/volume-expansion-mode"] != "offline",
	}, nil
}

type PersistentVolumeClaimFilter struct {
	Name            string
	RequestsStorage int64
	PromName        string
	StorageClass    string
	ResizePending   bool
}

func applyPersistentVolumeClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pvc = &corev1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	pvcSizeInBytes, ok := pvc.Spec.Resources.Requests.Storage().AsInt64()
	if !ok {
		return nil, errors.New(fmt.Sprintf("cannot get .Spec.Resources.Requests from PersistentVolumeClaim %s", pvc.Name))
	}

	resizePending := false
	for _, condition := range pvc.Status.Conditions {
		if condition.Type == "Resizing" || condition.Type == "FileSystemResizePending" {
			if condition.Status == "True" {
				resizePending = true
			}
			break
		}
	}

	return PersistentVolumeClaimFilter{
		Name:            pvc.Name,
		RequestsStorage: pvcSizeInBytes / 1024 / 1024 / 1024,
		PromName:        pvc.Labels["prometheus"],
		StorageClass:    *pvc.Spec.StorageClassName,
		ResizePending:   resizePending,
	}, nil
}

type PodFilter struct {
	Name           string
	Namespace      string
	PodScheduled   bool
	ContainerReady bool
	PromName       string
}

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod = &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	podScheduled := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "PodScheduled" {
			if condition.Status == "True" {
				podScheduled = true
			}
			break
		}
	}

	containerReady := false
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "prometheus" {
			containerReady = status.Ready
			break
		}
	}

	return PodFilter{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		PodScheduled:   podScheduled,
		ContainerReady: containerReady,
		PromName:       pod.Labels["prometheus"],
	}, nil
}

type PromFilter struct {
	Name string
}

func applyPromFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var prom = &monitoringv1.Prometheus{}
	err := sdk.FromUnstructured(obj, prom)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return PodFilter{
		Name: prom.Name,
	}, nil
}

func prometheusDisk(input *go_hook.HookInput, dc dependency.Container) error {
	podDeletionFlag := false
	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaimFilter)
		if pvc.ResizePending {
			podName := fmt.Sprintf("prometheus-%s-%s", pvc.PromName, pvc.Name[len(pvc.Name)-1:])
			for _, pObj := range input.Snapshots["pods"] {
				pod := pObj.(PodFilter)
				if podName == pod.Name && pod.PodScheduled {
					input.LogEntry.Infof("PersistentVolumeClaim %s in FileSystemResizePending state, deletion pod %s to complete resize", pvc.Name, podName)
					input.PatchCollector.Delete("v1", "Pod", "d8-monitoring", podName)
					podDeletionFlag = true
				}
			}
		}
	}

	if podDeletionFlag {
		return nil
	}

	isVolumeExpansionAllowed := func(scName string) bool {
		scs := input.Snapshots["scs"]
		for _, obj := range scs {
			sc := obj.(StorageClassFilter)
			if scName == sc.Name {
				return sc.AllowVolumeExpansion
			}
		}
		return false
	}

	proms := input.Snapshots["proms"]
	for _, prom := range proms {
		promName := prom.(PromFilter).Name

		var diskSize int64  // GiB
		var retention int64 // GiB

		if len(input.Snapshots["pvcs"]) == 0 {
			effectiveStorageClass := input.Values.Get(fmt.Sprintf("prometheus.internal.prometheus%s.effectiveStorageClass", promName)).String()
			// TODO
			input.LogEntry.Infof("prometheus.internal.prometheus%s.effectiveStorageClass: %s", promName, effectiveStorageClass)
			if effectiveStorageClass != "false" && isVolumeExpansionAllowed(effectiveStorageClass) {
				diskSize = 15
				retention = 10
			} else {
				diskSize = 30
				retention = 25
			}
		} else {
			var fsSize int64 // GiB
			var fsUsed int   // %

			var allowVolumeExpansion bool

			diskResizeLimit := input.ConfigValues.Get(fmt.Sprintf("prometheus.%sMaxDiskSizeGigabytes", promName)).Int()

			// find maximum PVC size
			pvcs := input.Snapshots["pvcs"]
			for _, obj := range pvcs {
				pvc := obj.(PersistentVolumeClaimFilter)

				if pvc.PromName != promName {
					continue
				}

				if diskSize == 0 {
					diskSize = pvc.RequestsStorage
					continue
				}

				if diskSize < pvc.RequestsStorage {
					diskSize = pvc.RequestsStorage
				}

				allowVolumeExpansion = isVolumeExpansionAllowed(pvc.StorageClass)
			}

			// TODO max
			pods := input.Snapshots["pods"]
			for _, obj := range pods {
				pod := obj.(PodFilter)
				if pod.PromName == promName && pod.PodScheduled && pod.ContainerReady {
					containerName := "prometheus"
					command := "df -PBG /prometheus/"
					output, _, err := execToPodThroughAPI(dc, command, containerName, pod.Name, pod.Namespace)
					if err != nil {
						input.LogEntry.Warnf("%s: %s", pod.Name, err.Error())
					} else {
						for _, s := range strings.Split(output, "\n") {
							if strings.Contains(s, "prometheus") {
								fsSize, _ = strconv.ParseInt(strings.Fields(s)[1], 10, 64)
								fsUsed, _ = strconv.Atoi(strings.Trim(strings.Fields(s)[4], "%"))
								break
							}
						}
					}
				}
			}

			if diskSize < fsSize {
				diskSize = fsSize
			}

			if allowVolumeExpansion && fsUsed > 77 {
				newDiskSize := diskSize + 5

				if newDiskSize <= diskResizeLimit {
					diskSize = newDiskSize
					patch := map[string]interface{}{
						"spec": map[string]interface{}{
							"resources": map[string]interface{}{
								"requests": map[string]string{
									"storage": fmt.Sprintf("%sGi", strconv.FormatInt(diskSize, 64)),
								},
							},
						},
					}

					for _, obj := range input.Snapshots["pvcs"] {
						pvc := obj.(PersistentVolumeClaimFilter)
						if pvc.PromName == promName {
							input.PatchCollector.MergePatch(patch, "v1", "PersistentVolumeClaim", "d8-monitoring", pvc.Name)
						}
					}

				}
			}
		}

		// TODO
		retention = diskSize * 8 / 10

		diskSizePath := fmt.Sprintf("prometheus.internal.prometheus%s.diskSizeGigabytes", promName)
		retentionPath := fmt.Sprintf("prometheus.internal.prometheus%s.retentionGigabytes", promName)

		input.Values.Set(diskSizePath, diskSize)
		input.Values.Set(retentionPath, retention)

	}

	return nil
}

func execToPodThroughAPI(dc dependency.Container, command, containerName, podName, namespace string) (string, string, error) {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return "", "", err
	}

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
