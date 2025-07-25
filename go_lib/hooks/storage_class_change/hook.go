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

package storage_class_change

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Args struct {
	ModuleName                    string `json:"moduleName"`
	Namespace                     string `json:"namespace"`
	LabelSelectorKey              string `json:"labelSelectorKey"`
	LabelSelectorValue            string `json:"labelSelectorValue"`
	ObjectKind                    string `json:"objectKind"`
	ObjectName                    string `json:"objectName"`
	InternalValuesSubPath         string `json:"internalValuesSubPath,omitempty"`
	D8ConfigStorageClassParamName string `json:"d8ConfigStorageClassParamName,omitempty"`

	// if return value is false - hook will stop its execution
	// if return value is true - hook will continue
	BeforeHookCheck func(input *go_hook.HookInput) bool
}

func RegisterHook(args Args) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "pvcs",
				ApiVersion: "v1",
				Kind:       "PersistentVolumeClaim",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{args.Namespace},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						args.LabelSelectorKey: args.LabelSelectorValue,
					},
				},
				FilterFunc: applyPVCFilter,
			},
			{
				Name:       "pods",
				ApiVersion: "v1",
				Kind:       "Pod",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{args.Namespace},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						args.LabelSelectorKey: args.LabelSelectorValue,
					},
				},
				FilterFunc: applyPodFilter,
			},
			{
				Name:       "default_sc",
				ApiVersion: "storage.k8s.io/v1",
				Kind:       "Storageclass",
				FilterFunc: applyDefaultStorageClassFilter,
			},
		},
	}, dependency.WithExternalDependencies(storageClassChange(args)))
}

type PVC struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	StorageClassName string `json:"storageClassName"`
	IsDeleted        bool   `json:"isDeleted"`
}

func applyPVCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pvc := &corev1.PersistentVolumeClaim{}

	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var isDeleted bool
	if pvc.DeletionTimestamp != nil {
		isDeleted = true
	}

	return PVC{
		Name:             pvc.Name,
		Namespace:        pvc.Namespace,
		StorageClassName: *pvc.Spec.StorageClassName,
		IsDeleted:        isDeleted,
	}, nil
}

type DefaultStorageClass struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

func applyDefaultStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storagev1.StorageClass{}

	err := sdk.FromUnstructured(obj, storageClass)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var isDefault bool

	if storageClass.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true" {
		isDefault = true
	}

	if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
		isDefault = true
	}

	return DefaultStorageClass{
		Name:      storageClass.Name,
		IsDefault: isDefault,
	}, nil
}

type Pod struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	PVCName   string          `json:"PVCName"`
	Phase     corev1.PodPhase `json:"phase"`
}

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}

	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var podPVCName string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			podPVCName = volume.PersistentVolumeClaim.ClaimName
		}
	}

	return Pod{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		PVCName:   podPVCName,
		Phase:     pod.Status.Phase,
	}, nil
}

// effective storage class is the target storage class. If it changes, the PVC will be recreated.
func calculateEffectiveStorageClass(input *go_hook.HookInput, args Args, currentStorageClass string) (string, error) {
	var effectiveStorageClass string

	defaultSCs, err := sdkobjectpatch.UnmarshalToStruct[DefaultStorageClass](input.NewSnapshots, "default_sc")
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal default_sc snapshot: %w", err)
	}
	for _, sc := range defaultSCs {
		if sc.IsDefault {
			effectiveStorageClass = sc.Name
			break
		}
	}

	if input.ConfigValues.Exists("global.modules.storageClass") {
		effectiveStorageClass = input.ConfigValues.Get("global.modules.storageClass").String()
	}

	// storage class from pvc
	if currentStorageClass != "" {
		effectiveStorageClass = currentStorageClass
	}

	var configValuesPath = fmt.Sprintf("%s.storageClass", args.ModuleName)

	if args.D8ConfigStorageClassParamName != "" {
		configValuesPath = fmt.Sprintf("%s.%s", args.ModuleName, args.D8ConfigStorageClassParamName)
	}

	if input.ConfigValues.Exists(configValuesPath) {
		effectiveStorageClass = input.ConfigValues.Get(configValuesPath).String()
	}

	var internalValuesPath = fmt.Sprintf("%s.internal.effectiveStorageClass", strcase.ToLowerCamel(args.ModuleName))

	if args.InternalValuesSubPath != "" {
		internalValuesPath = fmt.Sprintf("%s.internal.%s.effectiveStorageClass", strcase.ToLowerCamel(args.ModuleName), args.InternalValuesSubPath)
	}

	emptydirUsageMetricValue := 0.0
	if len(effectiveStorageClass) == 0 || effectiveStorageClass == "false" {
		input.Values.Set(internalValuesPath, false)
		emptydirUsageMetricValue = 1.0
	} else {
		input.Values.Set(internalValuesPath, effectiveStorageClass)
	}

	input.MetricsCollector.Set(
		"d8_emptydir_usage",
		emptydirUsageMetricValue,
		map[string]string{
			"namespace":   args.Namespace,
			"module_name": args.ModuleName,
		},
	)

	return effectiveStorageClass, nil
}

func storageClassChangeWithArgs(input *go_hook.HookInput, dc dependency.Container, args Args) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	pvcs, err := sdkobjectpatch.UnmarshalToStruct[PVC](input.NewSnapshots, "pvcs")
	if err != nil {
		return fmt.Errorf("cannot unmarshal pvcs snapshot: %w", err)
	}

	pods, err := sdkobjectpatch.UnmarshalToStruct[Pod](input.NewSnapshots, "pods")
	if err != nil {
		return fmt.Errorf("cannot unmarshal pods snapshot: %w", err)
	}

	findPodByPVCName := func(pvcName string) (Pod, error) {
		for _, pod := range pods {
			if pod.PVCName == pvcName {
				return pod, nil
			}
		}
		return Pod{}, fmt.Errorf("pod with volume name [%s] not found", pvcName)
	}

	var existingPvcs []PVC
	for _, pvc := range pvcs {
		if !pvc.IsDeleted {
			existingPvcs = append(existingPvcs, pvc)
			continue
		}

		pod, err := findPodByPVCName(pvc.Name)
		if err == nil {
			// if someone deleted pvc then evict the pod.
			err = kubeClient.CoreV1().Pods(pod.Namespace).Evict(context.TODO(), &v1beta1.Eviction{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name},
			})
			input.Logger.Info("evicting Pod due to PVC stuck in Terminating state", slog.String("namespace", pod.Namespace), slog.String("pod_name", pod.Name), slog.String("pvc_name", pvc.Name))

			if err != nil {
				input.Logger.Info("can't Evict Pod", slog.String("namespace", pod.Namespace), slog.String("name", pod.Name), log.Err(err))
			}
		}
	}

	var currentStorageClass string
	if len(existingPvcs) > 0 {
		currentStorageClass = existingPvcs[0].StorageClassName
	}

	effectiveStorageClass, err := calculateEffectiveStorageClass(input, args, currentStorageClass)
	if err != nil {
		return err
	}
	if !storageClassesAreEqual(currentStorageClass, effectiveStorageClass) {
		wasPvc := !isEmptyOrFalseStr(currentStorageClass)
		if wasPvc {
			for _, pvc := range existingPvcs {
				input.Logger.Info("PVC StorageClass changed. Deleting PersistentVolumeClaim", slog.String("namespace", pvc.Namespace), slog.String("name", pvc.Name))
				err = kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
				if err != nil {
					input.Logger.Error("failed to delete PVC", log.Err(err))
				}
			}
		}

		input.Logger.Info("StorageClass changed. Deleting objects", slog.String("namespace", args.Namespace), slog.String("object_kind", args.ObjectKind), slog.String("name", args.ObjectName))
		switch args.ObjectKind {
		case "Prometheus":
			err = kubeClient.Dynamic().Resource(schema.GroupVersionResource{
				Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses.monitoring.coreos.com",
			}).Namespace(args.Namespace).Delete(context.TODO(), args.ObjectName, metav1.DeleteOptions{})
		case "StatefulSet":
			err = kubeClient.AppsV1().StatefulSets(args.Namespace).Delete(context.TODO(), args.ObjectName, metav1.DeleteOptions{})
		default:
			input.Logger.Log(context.Background(), log.LevelFatal.Level(), "unknown object kind")
			panic("unknown object kind")
		}

		if err != nil && !errors.IsNotFound(err) {
			input.Logger.Error(err.Error())
		}
	}
	return nil
}

func storageClassesAreEqual(sc1, sc2 string) bool {
	if sc1 == sc2 {
		return true
	}
	return isEmptyOrFalseStr(sc1) && isEmptyOrFalseStr(sc2)
}

// isEmptyOrFalseStr returns true if sc is empty string or "false". For storage class values or
// configuration, empty strings and "false" mean the same: no storage class specified. "false" is
// set by humans, while absent values resolve to empty strings.
func isEmptyOrFalseStr(sc string) bool {
	return sc == "" || sc == "false"
}

func storageClassChange(args Args) func(input *go_hook.HookInput, dc dependency.Container) error {
	return func(input *go_hook.HookInput, dc dependency.Container) error {
		if args.BeforeHookCheck != nil && !args.BeforeHookCheck(input) {
			return nil
		}
		err := storageClassChangeWithArgs(input, dc, args)
		if err != nil {
			return err
		}
		return nil
	}
}
