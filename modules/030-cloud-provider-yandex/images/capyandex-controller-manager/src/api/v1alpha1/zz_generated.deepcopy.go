package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func (in *YandexMachineSpec) DeepCopyInto(out *YandexMachineSpec) {
	*out = *in
	if in.NetworkInterfaces != nil {
		out.NetworkInterfaces = make([]YandexNetworkInterface, len(in.NetworkInterfaces))
		copy(out.NetworkInterfaces, in.NetworkInterfaces)
	}
	if in.SchedulingPolicy != nil {
		p := *in.SchedulingPolicy
		out.SchedulingPolicy = &p
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string, len(in.Labels))
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	}
	if in.Metadata != nil {
		out.Metadata = make(map[string]string, len(in.Metadata))
		for key, val := range in.Metadata {
			out.Metadata[key] = val
		}
	}
}

func (in *YandexCluster) DeepCopyInto(out *YandexCluster) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec.ZoneToSubnetIDMap != nil {
		out.Spec.ZoneToSubnetIDMap = make(map[string]string, len(in.Spec.ZoneToSubnetIDMap))
		for key, val := range in.Spec.ZoneToSubnetIDMap {
			out.Spec.ZoneToSubnetIDMap[key] = val
		}
	}
}

func (in *YandexCluster) DeepCopy() *YandexCluster {
	if in == nil {
		return nil
	}
	out := new(YandexCluster)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexCluster) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *YandexClusterList) DeepCopyInto(out *YandexClusterList) {
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]YandexCluster, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *YandexClusterList) DeepCopy() *YandexClusterList {
	if in == nil {
		return nil
	}
	out := new(YandexClusterList)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexClusterList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *YandexMachine) DeepCopyInto(out *YandexMachine) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec.NetworkInterfaces != nil {
		out.Spec.NetworkInterfaces = make([]YandexNetworkInterface, len(in.Spec.NetworkInterfaces))
		copy(out.Spec.NetworkInterfaces, in.Spec.NetworkInterfaces)
	}
	if in.Spec.SchedulingPolicy != nil {
		p := *in.Spec.SchedulingPolicy
		out.Spec.SchedulingPolicy = &p
	}
	if in.Spec.Labels != nil {
		out.Spec.Labels = make(map[string]string, len(in.Spec.Labels))
		for key, val := range in.Spec.Labels {
			out.Spec.Labels[key] = val
		}
	}
	if in.Spec.Metadata != nil {
		out.Spec.Metadata = make(map[string]string, len(in.Spec.Metadata))
		for key, val := range in.Spec.Metadata {
			out.Spec.Metadata[key] = val
		}
	}
	if in.Status.Addresses != nil {
		out.Status.Addresses = make([]clusterv1.MachineAddress, len(in.Status.Addresses))
		copy(out.Status.Addresses, in.Status.Addresses)
	}
	if in.Status.FailureReason != nil {
		reason := *in.Status.FailureReason
		out.Status.FailureReason = &reason
	}
	if in.Status.FailureMessage != nil {
		message := *in.Status.FailureMessage
		out.Status.FailureMessage = &message
	}
	if in.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
		copy(out.Status.Conditions, in.Status.Conditions)
	}
}

func (in *YandexMachine) DeepCopy() *YandexMachine {
	if in == nil {
		return nil
	}
	out := new(YandexMachine)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexMachine) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *YandexMachineList) DeepCopyInto(out *YandexMachineList) {
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]YandexMachine, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *YandexMachineList) DeepCopy() *YandexMachineList {
	if in == nil {
		return nil
	}
	out := new(YandexMachineList)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexMachineList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *YandexMachineTemplate) DeepCopyInto(out *YandexMachineTemplate) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec.Template.ObjectMeta.Annotations != nil {
		out.Spec.Template.ObjectMeta.Annotations = make(map[string]string, len(in.Spec.Template.ObjectMeta.Annotations))
		for key, val := range in.Spec.Template.ObjectMeta.Annotations {
			out.Spec.Template.ObjectMeta.Annotations[key] = val
		}
	}
	if in.Spec.Template.ObjectMeta.Labels != nil {
		out.Spec.Template.ObjectMeta.Labels = make(map[string]string, len(in.Spec.Template.ObjectMeta.Labels))
		for key, val := range in.Spec.Template.ObjectMeta.Labels {
			out.Spec.Template.ObjectMeta.Labels[key] = val
		}
	}
	in.Spec.Template.Spec.DeepCopyInto(&out.Spec.Template.Spec)
}

func (in *YandexMachineTemplate) DeepCopy() *YandexMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(YandexMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexMachineTemplate) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *YandexMachineTemplateList) DeepCopyInto(out *YandexMachineTemplateList) {
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]YandexMachineTemplate, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *YandexMachineTemplateList) DeepCopy() *YandexMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(YandexMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

func (in *YandexMachineTemplateList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}
