package library

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
	hookExecResult, ok := actual.(*HookExecutionResult)
	if !ok {
		return false, fmt.Errorf("ExecuteSuccessfully matcher expects a *HookExecutionResult")
	}

	if hookExecResult.Session.ExitCode() == matcher.path {
		return true, nil
	}

	return false, nil
}

func (matcher *successfulExecutionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto be zero", actual.(*HookExecutionResult).Session.ExitCode())
}

func (matcher *successfulExecutionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto not be zero", actual.(*HookExecutionResult).Session.ExitCode())
}

func ValuesHasKey(path string) types.GomegaMatcher {
	return &valuesHasKeyMatcher{
		path: path,
	}
}

type valuesHasKeyMatcher struct {
	path string
}

func (matcher *valuesHasKeyMatcher) Match(actual interface{}) (success bool, err error) {
	hookExecResult, ok := actual.(*HookExecutionResult)
	if !ok {
		return false, fmt.Errorf("ValuesHasKey matcher expects a *HookExecutionResult")
	}

	result := hookExecResult.PatchedValuesGet(matcher.path)

	if result.Exists() {
		return true, nil
	}

	return false, nil
}

func (matcher *valuesHasKeyMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto contain key by path %q", actual.(*HookExecutionResult).PatchedValues, matcher.path)
}

func (matcher *valuesHasKeyMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto not contain key by path %q", actual.(*HookExecutionResult).PatchedValues, matcher.path)
}

func ConfigValuesHasKey(path string) types.GomegaMatcher {
	return &сonfigValuesHasKeyMatcher{
		expected: path,
	}
}

type сonfigValuesHasKeyMatcher struct {
	expected string
}

func (matcher *сonfigValuesHasKeyMatcher) Match(actual interface{}) (success bool, err error) {
	hookExecResult, ok := actual.(*HookExecutionResult)
	if !ok {
		return false, fmt.Errorf("ConfigValuesHasKey matcher expects a *HookExecutionResult")
	}

	result := hookExecResult.PatchedConfigValuesGet(matcher.expected)

	if result.Exists() {
		return true, nil
	}

	return false, nil
}

func (matcher *сonfigValuesHasKeyMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto to contain key by path %q", actual.(*HookExecutionResult).PatchedConfigValues, matcher.expected)
}

func (matcher *сonfigValuesHasKeyMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\nto to not contain key by path %q", actual.(*HookExecutionResult).PatchedConfigValues, matcher.expected)
}

func ValuesKeyEquals(path, value string) types.GomegaMatcher {
	return &valuesKeyEqualsMatcher{
		path:  path,
		value: value,
	}
}

type valuesKeyEqualsMatcher struct {
	path  string
	value string
}

func (matcher *valuesKeyEqualsMatcher) Match(actual interface{}) (success bool, err error) {
	hookExecResult, ok := actual.(*HookExecutionResult)
	if !ok {
		return false, fmt.Errorf("ValuesKeyEquals matcher expects a *HookExecutionResult")
	}

	result := hookExecResult.PatchedValuesGet(matcher.path)

	if result.String() == matcher.value {
		return true, nil
	}

	return false, nil
}

func (matcher *valuesKeyEqualsMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\n to have key by path %q that equals %q", actual.(*HookExecutionResult).PatchedValues, matcher.path, matcher.value)
}

func (matcher *valuesKeyEqualsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\n to not have key by path %q that equals %q", actual.(*HookExecutionResult).PatchedValues, matcher.path, matcher.value)
}

func ConfigValuesKeyEquals(path, value string) types.GomegaMatcher {
	return &configValuesKeyEqualsMatcher{
		path:  path,
		value: value,
	}
}

type configValuesKeyEqualsMatcher struct {
	path  string
	value string
}

func (matcher *configValuesKeyEqualsMatcher) Match(actual interface{}) (success bool, err error) {
	hookExecResult, ok := actual.(*HookExecutionResult)
	if !ok {
		return false, fmt.Errorf("ConfigValuesKeyEquals matcher expects a *HookExecutionResult")
	}

	result := hookExecResult.PatchedConfigValuesGet(matcher.path)

	if result.String() == matcher.value {
		return true, nil
	}

	return false, nil
}

func (matcher *configValuesKeyEqualsMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\n to have key by path %q that equals %q", actual.(*HookExecutionResult).PatchedConfigValues, matcher.path, matcher.value)
}

func (matcher *configValuesKeyEqualsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%v\n to not have key by path %q that equals %q", actual.(*HookExecutionResult).PatchedConfigValues, matcher.path, matcher.value)
}
