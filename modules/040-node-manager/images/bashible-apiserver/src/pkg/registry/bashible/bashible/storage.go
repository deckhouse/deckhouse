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

package bashible

import (
	"fmt"
	"os"
	"path"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
	"bashible-apiserver/pkg/template"
)

const templateName = "bashible.sh.tpl"

// NewStorage returns storage object that will work against API services.
func NewStorage(rootDir string, bashibleContext template.Context) (*Storage, error) {
	templatePath := path.Join(rootDir, "bashible", templateName)

	tplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read template: %v", err)
	}

	storage := &Storage{
		templateContent: tplContent,
		templateName:    templateName,
		bashibleContext: bashibleContext,
	}

	return storage, nil
}

type Storage struct {
	templateContent []byte
	templateName    string
	bashibleContext template.Context
}

// Render renders single script content by name
func (s Storage) Render(name string) (runtime.Object, error) {
	ngName, err := template.TransformName(name)
	if err != nil {
		return nil, fmt.Errorf("fail transform name: %v", err)
	}
	data, err := s.getContext(ngName)
	if err != nil {
		return nil, fmt.Errorf("cannot get context: %v", err)
	}
	r, err := template.RenderTemplate(templateName, s.templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("cannot render template: %v", err)
	}

	obj := bashible.Bashible{}
	obj.ObjectMeta.Name = ngName
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = map[string]string{}
	obj.Data[r.FileName] = r.Content.String()

	if checksum, ok := s.bashibleContext.GetConfigurationChecksum(ngName); ok {
		if obj.Annotations == nil {
			obj.Annotations = map[string]string{}
		}
		obj.Annotations["bashible.deckhouse.io/configuration-checksum"] = checksum
	}

	return &obj, nil
}

func (s Storage) getContext(name string) (map[string]interface{}, error) {
	contextKey, err := template.GetBashibleContextKey(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context key: %v", err)
	}

	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get context data: %v", err)
	}

	return context, nil
}

func (s Storage) New() runtime.Object {
	return &bashible.Bashible{}
}

func (s Storage) NewList() runtime.Object {
	return &bashible.BashibleList{}
}
