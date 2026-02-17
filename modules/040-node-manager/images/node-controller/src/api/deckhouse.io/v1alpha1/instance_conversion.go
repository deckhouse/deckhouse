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
	v1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this Instance (v1alpha1) to the hub version (v1alpha2).
func (src *Instance) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.Instance)
	dst.ObjectMeta = src.ObjectMeta
	dst.TypeMeta = src.TypeMeta
	dst.Spec = convertInstanceSpecToV1Alpha2(src.Status)
	dst.Status = convertInstanceStatusToV1Alpha2(src.Status)
	return nil
}

// ConvertTo converts this InstanceList (v1alpha1) to the hub version (v1alpha2).
func (src *InstanceList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.InstanceList)
	dst.TypeMeta = src.TypeMeta
	dst.ListMeta = src.ListMeta
	dst.Items = make([]v1alpha2.Instance, len(src.Items))
	for i := range src.Items {
		if err := src.Items[i].ConvertTo(&dst.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// ConvertFrom converts from the hub version (v1alpha2) to this version (v1alpha1).
func (dst *Instance) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.Instance)
	dst.ObjectMeta = src.ObjectMeta
	dst.TypeMeta = src.TypeMeta
	dst.Status = convertInstanceStatusFromV1Alpha2(src.Spec, src.Status)
	return nil
}

// ConvertFrom converts from the hub version (v1alpha2) to this version (v1alpha1).
func (dst *InstanceList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.InstanceList)
	dst.TypeMeta = src.TypeMeta
	dst.ListMeta = src.ListMeta
	dst.Items = make([]Instance, len(src.Items))
	for i := range src.Items {
		if err := dst.Items[i].ConvertFrom(&src.Items[i]); err != nil {
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
