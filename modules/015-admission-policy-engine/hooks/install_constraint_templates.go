/*
Copyright 2022 Flant JSC

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

package hooks

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, installConstraintTemplatesfunc)

func installConstraintTemplatesfunc(input *go_hook.HookInput) error {
	cTemplates, err := filepath.Glob("/deckhouse/modules/015-admission-policy-engine/templates/policies/*/*/ctemplate.yaml")
	if err != nil {
		return err
	}

	for _, cTemplatePath := range cTemplates {
		content, err := loadCTemplateFromFile(cTemplatePath)
		if err != nil {
			return err
		}

		input.PatchCollector.Create(content, object_patch.UpdateIfExists())
	}

	return nil
}

func loadCTemplateFromFile(crdFilePath string) ([]byte, error) {
	crdFile, err := os.Open(crdFilePath)
	if err != nil {
		return nil, err
	}

	defer crdFile.Close()

	content, err := ioutil.ReadAll(crdFile)
	if err != nil {
		return nil, err
	}

	return content, nil
}
