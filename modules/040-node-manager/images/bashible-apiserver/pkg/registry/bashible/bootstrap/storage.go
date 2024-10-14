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

package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
	"bashible-apiserver/pkg/template"
)

const templateName = "03-prepare-bashible.sh.tpl"

// NewStorage returns storage object that will work against API services.
func NewStorage(rootDir string, bashibleContext template.Context) (*Storage, error) {
	storage := &Storage{
		templatePath:    filepath.Join(rootDir, "bashible", "bootstrap", templateName),
		bashibleContext: bashibleContext,
	}

	return storage, nil
}

type Storage struct {
	templatePath    string
	bashibleContext template.Context
}

// Render renders single script content by name
func (s Storage) Render(ng string) (runtime.Object, error) {
	data, err := s.getContext(ng)
	if err != nil {
		return nil, fmt.Errorf("cannot get context: %v", err)
	}
	tplContent, err := os.ReadFile(s.templatePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read template: %v", err)
	}
	r, err := template.RenderTemplate(templateName, tplContent, data)
	if err != nil {
		return nil, fmt.Errorf("cannot render template: %v", err)
	}

	obj := bashible.Bootstrap{}
	obj.ObjectMeta.Name = ng
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Bootstrap = r.Content.String()

	return &obj, nil
}

func (s Storage) getContext(name string) (map[string]interface{}, error) {
	contextKey, err := template.GetBashibleContextKey(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context key: %v", err)
	}

	context, err := s.bashibleContext.GetBootstrapContext(contextKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get context data: %v", err)
	}

	return context, nil
}

func (s Storage) New() runtime.Object {
	return &bashible.Bootstrap{}
}

func (s Storage) NewList() runtime.Object {
	return &bashible.BootstrapList{}
}
