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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PhasesForCheck map[string]struct{}

type ByPhaseChecker struct {
	loggerProvider log.LoggerProvider
	phaseValues    PhasesForCheck
}

func NewByPhaseChecker(phaseValues PhasesForCheck, loggerProvider log.LoggerProvider) *ByPhaseChecker {
	return &ByPhaseChecker{
		loggerProvider: loggerProvider,
		phaseValues:    phaseValues,
	}
}

func (s *ByPhaseChecker) WaitAttemptsBeforeCheck() int {
	// i think it is enough
	return 3
}

func (s *ByPhaseChecker) IsReady(_ context.Context, resource *unstructured.Unstructured, resourceName string) (bool, error) {
	if len(s.phaseValues) == 0 {
		return false, fmt.Errorf("Internal error. No phase for check defined for resource %s", resourceName)
	}

	logger := log.SafeProvideLogger(s.loggerProvider)

	logNotReady := notFoundFuncDebugLogNotReady(logger, resourceName)
	castError := castErrorFuncForResource(resourceName, "")

	status := castKey[map[string]any](resource.Object, "status", logNotReady, castError)
	if !status.ok {
		return status.ReadyResult()
	}

	phaseRes := castKey[string](status.value, "phase", logNotReady, castError)
	if !phaseRes.ok {
		return phaseRes.ReadyResult()
	}

	phase := phaseRes.value
	_, res := s.phaseValues[phase]

	logger.LogDebugF("Found for %s currentStatus.phase '%s', result is %v.\n", resourceName, phase, res)

	return res, nil
}
