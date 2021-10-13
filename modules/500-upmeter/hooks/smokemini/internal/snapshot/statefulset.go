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

package snapshot

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultZone         = "NONE"
	DefaultStorageClass = "false"
)

type StatefulSet struct {
	Index string

	Zone         string
	Image        string
	Node         string
	StorageClass string
}

func NewStatefulSet(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sts := new(appsv1.StatefulSet)
	err := sdk.FromUnstructured(obj, sts)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize StatefulSet %q: %w", obj.GetName(), err)
	}

	var (
		anno = sts.GetAnnotations()
		node = ""
		zone = defaultZone
	)
	if nn, ok := anno["node"]; ok {
		// can also be found at .Spec.Template.Spec.Affinity.NodeAffinity...
		node = nn
	}
	if z, ok := anno["zone"]; ok {
		zone = z
	}

	index := IndexFromStatefulSetName(sts.GetName())
	image := sts.Spec.Template.Spec.Containers[0].Image

	var storageClassName string
	if sts.Spec.VolumeClaimTemplates != nil {
		for _, pvc := range sts.Spec.VolumeClaimTemplates {
			if pvc.GetName() == "disk" {
				if pvc.Spec.StorageClassName != nil {
					storageClassName = *pvc.Spec.StorageClassName
				}
				break
			}
		}
	}
	if storageClassName == "" {
		storageClassName = DefaultStorageClass
	}

	sss := StatefulSet{
		Index:        index.String(),
		Zone:         zone,
		Node:         node,
		StorageClass: storageClassName,
		Image:        image,
	}

	return sss, nil
}
