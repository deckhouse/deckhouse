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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	v1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	systemReserveMigrationCM    = "system-reserve-config-migration"
	systemReserveMigrationCMNew = "kubelet-resource-reservation-migration"
	systemReserveMigrationNS    = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ngs",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			FilterFunc:                   ngFilter,
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
		},
		{
			Name:                         "cm",
			ApiVersion:                   "v1",
			Kind:                         "ConfigMap",
			NameSelector:                 &types.NameSelector{MatchNames: []string{systemReserveMigrationCM}},
			FilterFunc:                   configMapName,
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
		},
		{
			Name:                         "cmNew",
			ApiVersion:                   "v1",
			Kind:                         "ConfigMap",
			NameSelector:                 &types.NameSelector{MatchNames: []string{systemReserveMigrationCMNew}},
			FilterFunc:                   configMapName,
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
		},
	},
}, systemReserve)

type NodeGroup struct {
	Name                    string
	ResourceReservationMode string
}

func ngFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng v1.NodeGroup
	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return "", err
	}

	return &NodeGroup{
		Name:                    ng.Name,
		ResourceReservationMode: string(ng.Spec.Kubelet.ResourceReservation.Mode),
	}, nil
}

type CM struct {
	Name string
}

func configMapName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	return &CM{Name: cm.Name}, nil
}

func systemReserve(input *go_hook.HookInput) error {
	if cmSnapshotNew := input.Snapshots["cmNew"]; len(cmSnapshotNew) > 0 {
		log.Debug("System reserved Nodes are already migrated, skipping...")
		return nil
	}

	ngsSnapshot := input.Snapshots["ngs"]
	for _, ngRaw := range ngsSnapshot {
		ng := ngRaw.(*NodeGroup)
		skipMigration := ng.ResourceReservationMode != ""
		input.LogEntry.Printf("NodeGroupName: %s, KubeletResourceReservationMode: %s, skipMigration: %t", ng.Name, ng.ResourceReservationMode, skipMigration)
		if skipMigration {
			continue
		}
		input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			objCopy := u.DeepCopy()
			err := unstructured.SetNestedField(objCopy.Object, "Off", "spec", "kubelet", "resourceReservation", "mode")
			if err != nil {
				return nil, err
			}
			return objCopy, nil
		}, "deckhouse.io/v1", "NodeGroup", "", ng.Name)
	}

	input.PatchCollector.Create(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      systemReserveMigrationCMNew,
			Namespace: systemReserveMigrationNS,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
	}, object_patch.IgnoreIfExists())

	if cmSnapshot := input.Snapshots["cm"]; len(cmSnapshot) > 0 {
		log.Debugf("Delete old migration configmap (d8-system/%s).", systemReserveMigrationCM)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", systemReserveMigrationCM)
	}

	return nil
}
