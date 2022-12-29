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

package resources

import (
	"strings"

	"github.com/flant/addon-operator/sdk"
	"k8s.io/api/flowcontrol/v1beta2"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
)

// ServiceAccountMustHaveFlowSchema fills linting error regarding FlowSchema
func ServiceAccountMustHaveFlowSchema(linter *rules.ObjectLinter) {
	scope := newLintingScope(linter.ObjectStore, linter.ErrorsList)

	for _, object := range scope.Objects() {
		// Skip non-pod controllers and modules which control VPA themselves
		if object.Unstructured.GetKind() != "ServiceAccount" {
			continue
		}

		lerr := ensureServiceAccountHaveFlowSchema(scope, object)
		linter.ErrorsList.Add(lerr)
	}
}

func ensureServiceAccountHaveFlowSchema(scope *lintingScope, sa storage.StoreObject) errors.LintRuleError {
	for _, object := range scope.Objects() {
		// Skip non-pod controllers and modules which control VPA themselves
		if object.Unstructured.GetKind() != "FlowSchema" {
			continue
		}

		var fs v1beta2.FlowSchema

		sdk.FromUnstructured(&object.Unstructured, &fs)

		if !strings.HasPrefix(fs.Spec.PriorityLevelConfiguration.Name, "cluster-") {
			continue
		}

		for _, r := range fs.Spec.Rules {
			for _, s := range r.Subjects {
				if s.ServiceAccount != nil {
					if s.ServiceAccount.Namespace == sa.Unstructured.GetNamespace() && s.ServiceAccount.Name == sa.Unstructured.GetName() {
						return errors.EmptyRuleError
					}
				}

				if s.Group != nil {
					if strings.HasSuffix(s.Group.Name, sa.Unstructured.GetNamespace()) {
						return errors.EmptyRuleError
					}
				}
			}
		}
	}

	return errors.NewLintRuleError(
		"SA001",
		sa.Identity(),
		nil,
		"Service account does not have matching FlowSchema")
}
