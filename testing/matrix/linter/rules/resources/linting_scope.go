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
