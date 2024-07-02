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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
	"bashible-apiserver/pkg/template"
)

// NewStorage returns a RESTStorage object that will work against API services.
func NewStorage(rootDir string, stepsStorage *template.StepsStorage, bashibleContext template.Context) (*StorageWithK8sBundles, error) {
	ngRenderer := template.NewStepsRenderer(stepsStorage, bashibleContext, rootDir, "node-group", template.GetNodegroupContextKey)
	k8sRenderer := template.NewStepsRenderer(stepsStorage, bashibleContext, rootDir, "all", template.GetVersionContextKey)

	return &StorageWithK8sBundles{
		ngRenderer:      ngRenderer,
		k8sRenderer:     k8sRenderer,
		bashibleContext: bashibleContext,
	}, nil
}

type StorageWithK8sBundles struct {
	ngRenderer      *template.StepsRenderer
	k8sRenderer     *template.StepsRenderer
	bashibleContext template.Context
}

// Render renders single script content by name which is expected to be of form {bundle}.{node-group-name}
// with hyphens as delimiters, e.g. `ubuntu-lts.master`.
func (s StorageWithK8sBundles) Render(ng string) (runtime.Object, error) {

	ngBundleData, err := s.ngRenderer.Render(ng)
	if err != nil {
		return nil, err
	}

	k8sBundleName, err := s.getK8sBundleName(ng)
	if err != nil {
		return nil, err
	}

	k8sBundleData, err := s.k8sRenderer.Render(k8sBundleName)
	if err != nil {
		return nil, err
	}

	data, err := s.merge(ngBundleData, k8sBundleData)
	if err != nil {
		return nil, err
	}

	obj := bashible.NodeGroupBundle{}
	obj.ObjectMeta.Name = ng
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = data

	return &obj, nil
}

func (s StorageWithK8sBundles) New() runtime.Object {
	return &bashible.NodeGroupBundle{}
}

func (s StorageWithK8sBundles) NewList() runtime.Object {
	return &bashible.NodeGroupBundleList{}
}

func (s StorageWithK8sBundles) getK8sBundleName(name string) (string, error) {
	contextKey, err := template.GetNodegroupContextKey(name)
	if err != nil {
		return "", err
	}

	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return "", err
	}

	versionObj, versionPresent := context["kubernetesVersion"]
	if !versionPresent {
		return "", fmt.Errorf("kubernetesVersion does not present in bundle context %s", contextKey)
	}
	k8sVer := strings.ReplaceAll(versionObj.(string), ".", "-")

	return fmt.Sprintf("%s", k8sVer), nil
}

func (s StorageWithK8sBundles) merge(ngBundleData, k8sBundleData map[string]string) (map[string]string, error) {
	for k, v := range k8sBundleData {
		if _, keyPresent := ngBundleData[k]; keyPresent {
			return nil, fmt.Errorf("%s already present in node-group bundle", k)
		}
		ngBundleData[k] = v
	}

	return ngBundleData, nil
}
