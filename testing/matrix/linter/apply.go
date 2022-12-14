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
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/resources"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

func ApplyLintRules(module utils.Module, values chartutil.Values, objectStore *storage.UnstructuredObjectStore) error {
	globalValues := values["Values"].(map[string]interface{})["global"].(map[string]interface{})

	enabledModules := set.New()
	for _, value := range globalValues["enabledModules"].([]interface{}) {
		enabledModules.Add(value.(string))
	}

	linter := rules.ObjectLinter{
		ObjectStore:    objectStore,
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
	resources.DaemonSetMustNotHavePDB(&linter)
	resources.NamespaceMustContainKubeRBACProxyCA(&linter)

	return linter.ErrorsList.ConvertToError()
}
