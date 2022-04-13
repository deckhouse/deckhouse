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
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/prometheus_disk",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "main",
			Crontab: "*/10 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "scs",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: applyStorageClassFilter,
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
	},
}, dependency.WithExternalDependencies(prometheusDisk))

type StorageClassFilter struct {
	Name                 string
	AllowVolumeExpansion bool
}

func applyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc = &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return StorageClassFilter{
		Name:                 sc.Name,
		AllowVolumeExpansion: pointer.BoolPtrDerefOr(sc.AllowVolumeExpansion, false) && sc.Annotations["storageclass.deckhouse.io/volume-expansion-mode"] != "offline",
	}, nil
}

type PersistentVolumeClaimFilter struct {
	Name            string
	RequestsStorage int64
	PromName        string
	StorageClass    string
	ResizePending   bool
	VolumeName      string
}

func applyPersistentVolumeClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pvc = &corev1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	pvcSizeInBytes, ok := pvc.Spec.Resources.Requests.Storage().AsInt64()
	if !ok {
		return nil, fmt.Errorf("cannot get .Spec.Resources.Requests from PersistentVolumeClaim %s", pvc.Name)
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
		VolumeName:      pvc.Spec.VolumeName,
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

func prometheusDisk(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	// checking pvc status for FileSystemResizePending or Resizing and restarting the pod if needed
	podDeletionFlag := false
	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaimFilter)
		if pvc.ResizePending {
			podName := fmt.Sprintf("prometheus-%s-%s", pvc.PromName, pvc.Name[len(pvc.Name)-1:])
			for _, pObj := range input.Snapshots["pods"] {
				pod := pObj.(PodFilter)
				if podName == pod.Name && pod.PodScheduled {
					input.LogEntry.Infof("PersistentVolumeClaim %s in FileSystemResizePending state, deletion pod %s to complete resize", pvc.Name, podName)
					// "300-prometheus/hooks/prometheus_disk.go Module hook failed, requeue task to retry after delay. Failed count is 1. Error: 1 error occurred:\n\t* Delete object v1/Pod/d8-monitoring/prometheus-main-0: timed out waiting for the condition\n\n"
					input.PatchCollector.Delete("v1", "Pod", "d8-monitoring", podName)
					podDeletionFlag = true
				}
			}
		}
	}

	if podDeletionFlag {
		return nil
	}

	proms := []string{"main", "longterm"}
	for _, promName := range proms {

		promNameForPath := strings.ToUpper(promName[0:1]) + promName[1:]

		var diskSize int64  // GiB
		var retention int64 // GiB

		pvcExists := false
		for _, obj := range input.Snapshots["pvcs"] {
			if obj.(PersistentVolumeClaimFilter).PromName == promName {
				pvcExists = true
				break
			}
		}

		// if there is no PVC, set the default values for diskSize and retention
		if !pvcExists {
			effectiveStorageClass := input.Values.Get(fmt.Sprintf("prometheus.internal.prometheus%s.effectiveStorageClass", promNameForPath)).String()
			input.LogEntry.Infof("prometheus.internal.prometheus%s.effectiveStorageClass: %s", promNameForPath, effectiveStorageClass)
			if effectiveStorageClass != "false" && isVolumeExpansionAllowed(input, effectiveStorageClass) {
				diskSize = 25
				retention = 22
			} else {
				diskSize = 30
				retention = 27
			}
			//	otherwise, we calculate diskSize and retention
		} else {
			diskResizeLimit := int64(300)
			maxDiskSizeConfigPath := fmt.Sprintf("prometheus.%sMaxDiskSizeGigabytes", promName)
			if input.ConfigValues.Exists(maxDiskSizeConfigPath) {
				diskResizeLimit = input.ConfigValues.Get(maxDiskSizeConfigPath).Int()
			}

			desiredSize := calcDesiredSize(input, kubeClient, promName)

			if desiredSize <= diskResizeLimit {
				diskSize = desiredSize
			}

			for _, obj := range input.Snapshots["pvcs"] {
				pvc := obj.(PersistentVolumeClaimFilter)
				if pvc.PromName != promName {
					continue
				}

				if pvc.RequestsStorage < diskSize {
					patch := makePatchRequestsStorage(diskSize)
					input.LogEntry.Infof("PersistentVolumeClaim %s size will be changed from %dGB to %dGB", pvc.Name, pvc.RequestsStorage, diskSize)
					input.PatchCollector.MergePatch(patch, "v1", "PersistentVolumeClaim", "d8-monitoring", pvc.Name)
					continue
				}
				if diskSize == 0 && pvc.RequestsStorage != 0 {
					diskSize = pvc.RequestsStorage
				}
			}

			retention = diskSize * 9 / 10 // 90%
		}

		diskSizePath := fmt.Sprintf("prometheus.internal.prometheus%s.diskSizeGigabytes", promNameForPath)
		retentionPath := fmt.Sprintf("prometheus.internal.prometheus%s.retentionGigabytes", promNameForPath)

		input.LogEntry.Debugf("diskSizePath: %s, diskSize: %d", diskSizePath, diskSize)

		input.Values.Set(diskSizePath, diskSize)
		input.Values.Set(retentionPath, retention)

	}

	return nil
}

func makePatchRequestsStorage(diskSize int64) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"resources": map[string]interface{}{
				"requests": map[string]string{
					"storage": fmt.Sprintf("%sGi", strconv.FormatInt(diskSize, 10)),
				},
			},
		},
	}
}

func isLocalStorage(input *go_hook.HookInput, kubeClient k8s.Client, promName string) bool {
	for _, obj := range input.Snapshots["pvcs"] {
		pvc := obj.(PersistentVolumeClaimFilter)

		if pvc.PromName != promName {
			continue
		}

		pvName := pvc.VolumeName

		pv, err := kubeClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
		if err != nil {
			return false
		}

		if pv.Spec.Local != nil {
			if len(pv.Spec.Local.Path) > 0 {
				return true
			}
		}
	}
	return false
}

func calcDesiredSize(input *go_hook.HookInput, kubeClient k8s.Client, promName string) (diskSize int64) {
	var allowVolumeExpansion bool

	// find maximum PVC size
	pvcs := input.Snapshots["pvcs"]
	for _, obj := range pvcs {
		pvc := obj.(PersistentVolumeClaimFilter)

		if pvc.PromName != promName {
			continue
		}

		allowVolumeExpansion = isVolumeExpansionAllowed(input, pvc.StorageClass)

		if diskSize == 0 || diskSize < pvc.RequestsStorage {
			diskSize = pvc.RequestsStorage
		}
	}

	// find maximum filesystem size and used space
	var fsSize int64 // GiB
	var fsUsed int   // %
	pods := input.Snapshots["pods"]
	for _, obj := range pods {
		pod := obj.(PodFilter)
		if pod.PromName == promName && pod.PodScheduled && pod.ContainerReady {
			podFsSize, podFsUsed := getFsSizeAndUsed(input, kubeClient, pod)
			input.LogEntry.Debugf("%s, fsSize: %d, fsUsed: %d", pod.Name, podFsSize, podFsUsed)

			input.MetricsCollector.Set(
				"d8_prometheus_fs_size",
				float64(podFsSize),
				map[string]string{
					"namespace": pod.Namespace,
					"pod_name":  pod.Name,
				},
				metrics.WithGroup("prometheus_disk_hook"),
			)

			input.MetricsCollector.Set(
				"d8_prometheus_fs_used",
				float64(podFsUsed),
				map[string]string{
					"namespace": pod.Namespace,
					"pod_name":  pod.Name,
				},
				metrics.WithGroup("prometheus_disk_hook"),
			)

			if podFsSize > fsSize {
				fsSize = podFsSize
			}

			if podFsUsed > fsUsed {
				fsUsed = podFsUsed
			}
		}
	}

	if diskSize < fsSize {
		diskSize = fsSize
	}

	if isLocalStorage(input, kubeClient, promName) && fsSize > 0 {
		diskSize = fsSize
	}

	input.LogEntry.Debugf("%s, allowVolumeExpansion: %t, fsUsed: %d", promName, allowVolumeExpansion, fsUsed)

	if allowVolumeExpansion && fsUsed > 80 {
		diskSize += 5
	}

	return
}

func isVolumeExpansionAllowed(input *go_hook.HookInput, scName string) bool {
	scs := input.Snapshots["scs"]
	for _, obj := range scs {
		sc := obj.(StorageClassFilter)
		if scName == sc.Name {
			return sc.AllowVolumeExpansion
		}
	}
	return false
}

func getFsSizeAndUsed(input *go_hook.HookInput, kubeClient k8s.Client, pod PodFilter) (fsSize int64, fsUsed int) {
	containerName := "prometheus"
	command := "df -PBG /prometheus/"
	output, _, err := execToPodThroughAPI(kubeClient, command, containerName, pod.Name, pod.Namespace)
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
