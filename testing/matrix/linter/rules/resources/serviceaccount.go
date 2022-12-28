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
	"github.com/flant/addon-operator/sdk"
	"k8s.io/api/flowcontrol/v1beta2"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
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

		ensureServiceAccountHaveFlowSchema(scope, object)
	}
}

func ensureServiceAccountHaveFlowSchema(scope *lintingScope, sa storage.StoreObject) {
	for _, object := range scope.Objects() {
		// Skip non-pod controllers and modules which control VPA themselves
		if object.Unstructured.GetKind() != "FlowSchema" {
			continue
		}

		var fs v1beta2.FlowSchema
		sdk.FromUnstructured(&object.Unstructured, &fs)
	}
}
