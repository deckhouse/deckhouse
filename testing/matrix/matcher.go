package matrix

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

type LintErrorOccurredMatcher struct {
}

func (matcher *LintErrorOccurredMatcher) Match(actual interface{}) (success bool, err error) {
	// is purely nil?
	if actual == nil {
		return false, nil
	}

	return true, nil
}

func (matcher *LintErrorOccurredMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected an error to have occurred.  Got:\n%s", format.Object(actual, 1))
}

func (matcher *LintErrorOccurredMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Linter error:\n\n%s\n", format.IndentString(actual.(error).Error(), 1))
}

func ErrorOccurred() types.GomegaMatcher {
	return &LintErrorOccurredMatcher{}
}
