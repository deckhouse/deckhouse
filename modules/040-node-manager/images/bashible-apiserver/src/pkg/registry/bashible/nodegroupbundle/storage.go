/*
Copyright 2023 Flant JSC

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
func NewStorage(rootDir string, stepsStorage *template.StepsStorage, bashibleContext template.Context) (*StorageWithK8sBundles, error) {
	ngRenderer := template.NewStepsRenderer(stepsStorage, bashibleContext, rootDir, "all", template.GetNodegroupContextKey)

	return &StorageWithK8sBundles{
		ngRenderer:      ngRenderer,
		bashibleContext: bashibleContext,
	}, nil
}

type StorageWithK8sBundles struct {
	ngRenderer      *template.StepsRenderer
	bashibleContext template.Context
}

// Render renders single script content by ng name.
func (s StorageWithK8sBundles) Render(ng string) (runtime.Object, error) {
	ngBundleData, err := s.ngRenderer.Render(ng)
	if err != nil {
		return nil, err
	}

	obj := bashible.NodeGroupBundle{}
	obj.ObjectMeta.Name = ng
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = ngBundleData

	return &obj, nil
}

func (s StorageWithK8sBundles) New() runtime.Object {
	return &bashible.NodeGroupBundle{}
}

func (s StorageWithK8sBundles) NewList() runtime.Object {
	return &bashible.NodeGroupBundleList{}
}
