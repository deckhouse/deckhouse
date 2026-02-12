/*
Copyright 2026 Flant JSC

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

package v1alpha1

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

const instanceConversionDataAnnotation = "node-controller.deckhouse.io/conversion-data"

// ConvertTo converts this Instance (v1alpha1) to the hub version (v1alpha2).
func (obj *Instance) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.Instance)
	dst.ObjectMeta = obj.ObjectMeta
	dst.TypeMeta = obj.TypeMeta
	dst.Spec = convertInstanceSpecToV1Alpha2(obj.Status)
	dst.Status = convertInstanceStatusToV1Alpha2(obj.Status)

	// Restore hub-only data preserved during a previous down-conversion.
	restored := &v1alpha2.Instance{}
	if ok, err := unmarshalInstanceHubData(obj, restored); err != nil || !ok {
		return err
	}

	// Fields below do not exist in v1alpha1 and are otherwise lost on round-trip.
	dst.Spec = restored.Spec
	dst.Status.MachineStatus = restored.Status.MachineStatus
	dst.Status.BashibleStatus = restored.Status.BashibleStatus
	dst.Status.Message = restored.Status.Message
	dst.Status.Conditions = restored.Status.Conditions
	if restored.Status.Phase != "" {
		dst.Status.Phase = restored.Status.Phase
	}

	return nil
}

// ConvertTo converts this InstanceList (v1alpha1) to the hub version (v1alpha2).
func (obj *InstanceList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.InstanceList)
	dst.TypeMeta = obj.TypeMeta
	dst.ListMeta = obj.ListMeta
	dst.Items = make([]v1alpha2.Instance, len(obj.Items))
	for i := range obj.Items {
		if err := obj.Items[i].ConvertTo(&dst.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// ConvertFrom converts from the hub version (v1alpha2) to this version (v1alpha1).
func (obj *Instance) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.Instance)
	obj.ObjectMeta = src.ObjectMeta
	obj.TypeMeta = src.TypeMeta
	obj.Status = convertInstanceStatusFromV1Alpha2(src.Spec, src.Status)

	// Preserve hub-only data in annotation to avoid lossy down-conversion.
	if err := marshalInstanceHubData(src, obj); err != nil {
		return err
	}

	return nil
}

func marshalInstanceHubData(src *v1alpha2.Instance, dst *Instance) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(src)
	if err != nil {
		return err
	}
	delete(u, "metadata")

	data, err := json.Marshal(u)
	if err != nil {
		return err
	}

	if dst.Annotations == nil {
		dst.Annotations = map[string]string{}
	}
	dst.Annotations[instanceConversionDataAnnotation] = string(data)

	return nil
}

func unmarshalInstanceHubData(src *Instance, dst *v1alpha2.Instance) (bool, error) {
	if src.Annotations == nil {
		return false, nil
	}

	data, ok := src.Annotations[instanceConversionDataAnnotation]
	if !ok {
		return false, nil
	}

	if err := json.Unmarshal([]byte(data), dst); err != nil {
		return false, err
	}

	delete(src.Annotations, instanceConversionDataAnnotation)
	if len(src.Annotations) == 0 {
		src.Annotations = nil
	}

	return true, nil
}

// ConvertFrom converts from the hub version (v1alpha2) to this version (v1alpha1).
func (obj *InstanceList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.InstanceList)
	obj.TypeMeta = src.TypeMeta
	obj.ListMeta = src.ListMeta
	obj.Items = make([]Instance, len(src.Items))
	for i := range src.Items {
		if err := obj.Items[i].ConvertFrom(&src.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func convertInstanceStatusToV1Alpha2(src InstanceStatus) v1alpha2.InstanceStatus {
	dst := v1alpha2.InstanceStatus{}

	if src.CurrentStatus.Phase != "" {
		dst.Phase = v1alpha2.InstancePhase(src.CurrentStatus.Phase)
	}

	return dst
}

func convertInstanceSpecToV1Alpha2(src InstanceStatus) v1alpha2.InstanceSpec {
	spec := v1alpha2.InstanceSpec{
		NodeRef: v1alpha2.NodeRef{
			Name: src.NodeRef.Name,
		},
	}

	if src.MachineRef.Kind != "" || src.MachineRef.APIVersion != "" || src.MachineRef.Name != "" || src.MachineRef.Namespace != "" {
		spec.MachineRef = &v1alpha2.MachineRef{
			Kind:       src.MachineRef.Kind,
			APIVersion: src.MachineRef.APIVersion,
			Name:       src.MachineRef.Name,
			Namespace:  src.MachineRef.Namespace,
		}
	}

	if src.ClassReference.Kind != "" || src.ClassReference.Name != "" {
		spec.ClassReference = &v1alpha2.ClassReference{
			Kind: src.ClassReference.Kind,
			Name: src.ClassReference.Name,
		}
	}

	return spec
}

func convertInstanceStatusFromV1Alpha2(srcSpec v1alpha2.InstanceSpec, srcStatus v1alpha2.InstanceStatus) InstanceStatus {
	status := InstanceStatus{
		NodeRef: NodeRef{
			Name: srcSpec.NodeRef.Name,
		},
	}

	if srcSpec.MachineRef != nil {
		status.MachineRef = MachineRef{
			Kind:       srcSpec.MachineRef.Kind,
			APIVersion: srcSpec.MachineRef.APIVersion,
			Name:       srcSpec.MachineRef.Name,
			Namespace:  srcSpec.MachineRef.Namespace,
		}
	}

	if srcSpec.ClassReference != nil {
		status.ClassReference = ClassReference{
			Kind: srcSpec.ClassReference.Kind,
			Name: srcSpec.ClassReference.Name,
		}
	}

	if srcStatus.Phase != "" {
		status.CurrentStatus.Phase = InstancePhase(srcStatus.Phase)
	}

	return status
}
