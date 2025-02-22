/*
Copyright 2025 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	WorkspaceKind     = "Workspace"
	WorkspaceResource = "workspaces"

	WorkspaceFinalizer = "workspaces.deckhouse.io/workspace-exists"
)

var _ runtime.Object = &Workspace{}

type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func (w *WorkspaceList) DeepCopyObject() runtime.Object {
	return w.DeepCopy()
}
func (w *WorkspaceList) DeepCopy() *WorkspaceList {
	if w == nil {
		return nil
	}
	newObj := new(WorkspaceList)
	w.DeepCopyInto(newObj)
	return newObj
}
func (w *WorkspaceList) DeepCopyInto(newObj *WorkspaceList) {
	*newObj = *w
	newObj.TypeMeta = w.TypeMeta
	w.ListMeta.DeepCopyInto(&newObj.ListMeta)
	if w.Items != nil {
		in, out := &w.Items, &newObj.Items
		*out = make([]Workspace, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status WorkspaceStatus `json:"status,omitempty"`
}

func (w *Workspace) DeepCopyObject() runtime.Object {
	return w.DeepCopy()
}
func (w *Workspace) DeepCopy() *Workspace {
	if w == nil {
		return nil
	}
	newObj := Workspace{}
	w.DeepCopyInto(&newObj)
	return &newObj
}
func (w *Workspace) DeepCopyInto(newObj *Workspace) {
	*newObj = *w
	newObj.TypeMeta = w.TypeMeta
	w.ObjectMeta.DeepCopyInto(&newObj.ObjectMeta)
	w.Status.DeepCopyInto(&newObj.Status)
}

type WorkspaceStatus struct {
	// Current state.
	State string `json:"state,omitempty"`
}

func (w *Workspace) SetState(state string) {
	w.Status.State = state
}

func (p *WorkspaceStatus) DeepCopy() *WorkspaceStatus {
	if p == nil {
		return nil
	}
	newObj := new(WorkspaceStatus)
	p.DeepCopyInto(newObj)
	return newObj
}
func (p *WorkspaceStatus) DeepCopyInto(newObj *WorkspaceStatus) {
	*newObj = *p
}
