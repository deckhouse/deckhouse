/*
Copyright 2017 The Kubernetes Authors.

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

package nodegroupbundle

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
	"bashible-apiserver/pkg/template"
)

// NewStorage returns a RESTStorage object that will work against API services.
func NewStorage(rootDir string, bashibleContext *template.Context) (*Storage, error) {
	renderer := template.NewStepsRenderer(bashibleContext, rootDir, "node-group", template.GetNodegroupContextKey)
	return &Storage{renderer}, nil
}

type Storage struct {
	renderer *template.StepsRenderer
}

// Render renders single script content by name which is expected to be of form {bundle}.{node-group-name}
// with hyphens as delimiters, e.g. `ubuntu-lts.master`.
func (s Storage) Render(name string) (runtime.Object, error) {
	data, err := s.renderer.Render(name)
	if err != nil {
		return nil, err
	}

	obj := bashible.NodeGroupBundle{}
	obj.ObjectMeta.Name = name
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = data

	return &obj, nil
}

func (s Storage) New() runtime.Object {
	return &bashible.NodeGroupBundle{}
}

func (s Storage) NewList() runtime.Object {
	return &bashible.NodeGroupBundleList{}
}
