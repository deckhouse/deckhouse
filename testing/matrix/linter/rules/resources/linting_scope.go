/*
Copyright 2021 Flant CJSC

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
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
)

// lintingScope is the wrapper for linting state
type lintingScope struct {
	// errors is linting error list from the outside that is to be fulfilled on the go
	errors *errors.LintRuleErrorsList

	// store keeps parsed module Objects, not to modify
	store *storage.UnstructuredObjectStore
}

// newLintingScope creates linting scope to iterate over module objects and gather errors
func newLintingScope(objectStore *storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList) *lintingScope {
	return &lintingScope{
		errors: lintRuleErrorsList,
		store:  objectStore,
	}
}

func (p *lintingScope) Objects() map[storage.ResourceIndex]storage.StoreObject {
	return p.store.Storage
}

// AddError repeats the signature of errors.NewLintRuleError to collect linting errors
func (p *lintingScope) AddError(id, objectID string, value interface{}, template string, a ...interface{}) {
	ruleErr := errors.NewLintRuleError(id, objectID, value, template, a...)
	p.errors.Add(ruleErr)
}
