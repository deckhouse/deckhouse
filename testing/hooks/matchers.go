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

func (matcher *successfulExecutionMatcher) Match(actual interface{}) (bool, error) {
	hec, ok := actual.(*HookExecutionConfig)
	if !ok {
		return false, fmt.Errorf("matcher ExecuteSuccessfully expects a *HookExecutionConfig")
	}

	// There is no Session for go hooks, so this matcher is just ignored
	if hec.GoHook != nil {
		if hec.GoHookError != nil {
			return false, nil // dont return hec.GoHookError here, its Matcher error, not execute func error
		}

		return true, nil
	}

	if hec.Session.ExitCode() == matcher.path {
		return true, nil
	}

	return false, nil
}

func (matcher *successfulExecutionMatcher) FailureMessage(actual interface{}) string {
	hec := actual.(*HookExecutionConfig)
	if hec.GoHook != nil && hec.GoHookError != nil {
		return fmt.Sprintf("Expected\n\t%v\n to be nil", hec.GoHookError)
	}

	return fmt.Sprintf("Expected\n\t%v\nto be zero", actual.(*HookExecutionConfig).Session.ExitCode())
}

func (matcher *successfulExecutionMatcher) NegatedFailureMessage(actual interface{}) string {
	hec := actual.(*HookExecutionConfig)
	if hec.GoHook != nil && hec.GoHookError == nil {
		return fmt.Sprintf("Expected\n\t%v\nnot to be nil", hec.GoHookError)
	}

	return fmt.Sprintf("Expected\n\t%v\nto not be zero", actual.(*HookExecutionConfig).Session.ExitCode())
}
