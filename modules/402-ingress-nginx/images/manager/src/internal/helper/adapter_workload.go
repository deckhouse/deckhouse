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

package helper

import (
	"fmt"

	kruiseappsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NativeDaemonSet struct {
	Obj *appsv1.DaemonSet
}

func (d NativeDaemonSet) Kind() string { return "DaemonSet" }
func (d NativeDaemonSet) NamespacedName() string {
	return fmt.Sprintf("%s/%s", d.Obj.Namespace, d.Obj.Name)
}
func (d NativeDaemonSet) GetGeneration() int64         { return d.Obj.Generation }
func (d NativeDaemonSet) GetObservedGeneration() int64 { return d.Obj.Status.ObservedGeneration }
func (d NativeDaemonSet) GetDesiredNumberScheduled() int32 {
	return d.Obj.Status.DesiredNumberScheduled
}

func (d NativeDaemonSet) GetCurrentNumberScheduled() int32 {
	return d.Obj.Status.CurrentNumberScheduled
}

func (d NativeDaemonSet) GetUpdatedNumberScheduled() int32 {
	return d.Obj.Status.UpdatedNumberScheduled
}
func (d NativeDaemonSet) GetNumberReady() int32                 { return d.Obj.Status.NumberReady }
func (d NativeDaemonSet) GetNumberAvailable() int32             { return d.Obj.Status.NumberAvailable }
func (d NativeDaemonSet) GetNumberUnavailable() int32           { return d.Obj.Status.NumberUnavailable }
func (d NativeDaemonSet) GetPodSelector() *metav1.LabelSelector { return d.Obj.Spec.Selector }
func (d NativeDaemonSet) GetNamespace() string                  { return d.Obj.Namespace }
func (d NativeDaemonSet) GetCreationTimestamp() metav1.Time     { return d.Obj.CreationTimestamp }

type AdvancedDaemonSet struct {
	Obj *kruiseappsv1alpha1.DaemonSet
}

func (d AdvancedDaemonSet) Kind() string { return "AdvancedDaemonSet" }
func (d AdvancedDaemonSet) NamespacedName() string {
	return fmt.Sprintf("%s/%s", d.Obj.Namespace, d.Obj.Name)
}
func (d AdvancedDaemonSet) GetGeneration() int64         { return d.Obj.Generation }
func (d AdvancedDaemonSet) GetObservedGeneration() int64 { return d.Obj.Status.ObservedGeneration }
func (d AdvancedDaemonSet) GetDesiredNumberScheduled() int32 {
	return d.Obj.Status.DesiredNumberScheduled
}

func (d AdvancedDaemonSet) GetCurrentNumberScheduled() int32 {
	return d.Obj.Status.CurrentNumberScheduled
}

func (d AdvancedDaemonSet) GetUpdatedNumberScheduled() int32 {
	return d.Obj.Status.UpdatedNumberScheduled
}
func (d AdvancedDaemonSet) GetNumberReady() int32                 { return d.Obj.Status.NumberReady }
func (d AdvancedDaemonSet) GetNumberAvailable() int32             { return d.Obj.Status.NumberAvailable }
func (d AdvancedDaemonSet) GetNumberUnavailable() int32           { return d.Obj.Status.NumberUnavailable }
func (d AdvancedDaemonSet) GetPodSelector() *metav1.LabelSelector { return d.Obj.Spec.Selector }
func (d AdvancedDaemonSet) GetNamespace() string                  { return d.Obj.Namespace }
func (d AdvancedDaemonSet) GetCreationTimestamp() metav1.Time     { return d.Obj.CreationTimestamp }
