/*
Copyright 2021 Flant JSC

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

package linter

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/resources"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

func ApplyLintRules(module utils.Module, values string, objectStore *storage.UnstructuredObjectStore) error {
	var v struct {
		Global struct {
			EnabledModules []string `yaml:"enabledModules"`
		} `yaml:"global"`
	}
	err := yaml.Unmarshal([]byte(values), &v)
	if err != nil {
		return fmt.Errorf("unable to parse global.enabledModules values section")
	}

	// Use map for faster lookups
	enabledModules := make(map[string]struct{}, len(v.Global.EnabledModules))
	for _, value := range v.Global.EnabledModules {
		enabledModules[value] = struct{}{}
	}

	linter := rules.ObjectLinter{
		ObjectStore:    objectStore,
		Values:         values,
		Module:         module,
		EnabledModules: enabledModules,
		ErrorsList:     &errors.LintRuleErrorsList{},
	}

	for _, object := range objectStore.Storage {
		linter.ApplyObjectRules(object)
		linter.ApplyContainerRules(object)
	}

	resources.ControllerMustHaveVPA(&linter)
	resources.ControllerMustHavePDB(&linter)

	return linter.ErrorsList.ConvertToError()
}
