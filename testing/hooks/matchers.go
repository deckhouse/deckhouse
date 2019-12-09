package hooks

import (
	"fmt"

	"github.com/onsi/gomega/types"
)

func ExecuteSuccessfully() types.GomegaMatcher {
	return &successfulExecutionMatcher{
		path: 0,
	}
}

type successfulExecutionMatcher struct {
	path int
}

func (matcher *successfulExecutionMatcher) Match(actual interface{}) (success bool, err error) {
	hec, ok := actual.(*HookExecutionConfig)
	if !ok {
		return false, fmt.Errorf("ExecuteSuccessfully matcher expects a *HookExecutionConfig")
	}

	if hec.Session.ExitCode() == matcher.path {
		return true, nil
	}

	return false, nil
}

func (matcher *successfulExecutionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto be zero", actual.(*HookExecutionConfig).Session.ExitCode())
}

func (matcher *successfulExecutionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto not be zero", actual.(*HookExecutionConfig).Session.ExitCode())
}
