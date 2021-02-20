package matrix

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

const (
	failureErrMessage = `Expected an error to occur. Got:
%s
`
	negatedLinterErrMessage = `Linter error:

%s
`
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
	return fmt.Sprintf(failureErrMessage, format.Object(actual, 1))
}

func (matcher *LintErrorOccurredMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(negatedLinterErrMessage, format.IndentString(actual.(error).Error(), 1))
}

func ErrorOccurred() types.GomegaMatcher {
	return &LintErrorOccurredMatcher{}
}
