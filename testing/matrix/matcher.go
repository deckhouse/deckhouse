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
