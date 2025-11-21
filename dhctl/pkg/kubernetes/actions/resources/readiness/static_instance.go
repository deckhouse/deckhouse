// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package readiness

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type StaticInstanceChecker struct {
	loggerProvider LoggerProvider
}

func NewStaticInstanceChecker(loggerProvider LoggerProvider) *StaticInstanceChecker {
	return &StaticInstanceChecker{
		loggerProvider: loggerProvider,
	}
}

func (s *StaticInstanceChecker) WaitAttemptsBeforeCheck() int {
	// because we will wait status and conditionsForCheck in every time we can do not wait many times
	return 1
}

func (s *StaticInstanceChecker) IsReady(_ context.Context, resource *unstructured.Unstructured, resourceName string) (bool, error) {
	logger := safeLoggerProvider(s.loggerProvider)

	logNotReady := notFoundFuncDebugLogNotReady(logger, resourceName)
	castError := castErrorFuncForResource(resourceName, "")

	status := castKey[map[string]any](resource.Object, "status", logNotReady, castError)
	if !status.ok {
		return status.ReadyResult()
	}

	currentStatus := castKey[map[string]any](status.value, "currentStatus", logNotReady, castError)
	if !currentStatus.ok {
		return currentStatus.ReadyResult()
	}

	phaseRes := castKey[string](currentStatus.value, "phase", logNotReady, castError)
	if !phaseRes.ok {
		return phaseRes.ReadyResult()
	}

	phase := phaseRes.value
	res := phase == "Running"

	logger.LogDebugF("Found for %s currentStatus.phase '%s', result is %v.\n", resourceName, phase, res)

	return res, nil
}
