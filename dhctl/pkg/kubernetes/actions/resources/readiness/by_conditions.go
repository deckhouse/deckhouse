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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// type -> type value
type Conditions map[string]string

type ByConditionsChecker struct {
	loggerProvider     log.LoggerProvider
	conditionsForCheck Conditions

	waitAttempts int

	checkAllConditions          bool
	readyIfNoStatusOrConditions bool
}

func NewByConditionsChecker(conditions Conditions, loggerProvider log.LoggerProvider) *ByConditionsChecker {
	return &ByConditionsChecker{
		loggerProvider:     loggerProvider,
		conditionsForCheck: conditions,
		waitAttempts:       5,

		checkAllConditions:          false,
		readyIfNoStatusOrConditions: false,
	}
}

func (c *ByConditionsChecker) WithCheckAll(v bool) *ByConditionsChecker {
	c.checkAllConditions = v
	return c
}

func (c *ByConditionsChecker) WithReadyIfNoStatusOrConditions(v bool) *ByConditionsChecker {
	c.readyIfNoStatusOrConditions = v
	return c
}

func (c *ByConditionsChecker) WithWaitAttempts(a int) *ByConditionsChecker {
	c.waitAttempts = a
	return c
}

func (c *ByConditionsChecker) WaitAttemptsBeforeCheck() int {
	return c.waitAttempts
}

func (c *ByConditionsChecker) IsReady(_ context.Context, resource *unstructured.Unstructured, resourceName string) (bool, error) {
	if len(c.conditionsForCheck) == 0 {
		return false, fmt.Errorf("Internal error: no conditionsForCheck found for resource %s", resourceName)
	}

	logger := log.SafeProvideLogger(c.loggerProvider)

	var logReadyOrNot castErrorFunc
	castError := castErrorFuncForResource(resourceName, "")

	if c.readyIfNoStatusOrConditions {
		logReadyOrNot = notFoundFuncDebugLogReady(logger, resourceName)
	} else {
		logReadyOrNot = notFoundFuncDebugLogNotReady(logger, resourceName)
	}

	status := castKey[map[string]any](resource.Object, "status", logReadyOrNot, castError)
	if !status.ok {
		return status.ReadyResult()
	}

	conditions := castKey[[]any](status.value, "conditions", logReadyOrNot, castError)
	if !conditions.ok {
		return conditions.ReadyResult()
	}

	conditionsResults := make(map[string]bool, len(c.conditionsForCheck))

	for indx, conditionRaw := range conditions.value {
		castErrorCondition := castErrorFuncForResource(resourceName, fmt.Sprintf("condition index %d", indx))

		conditionMap := castVal[map[string]any](conditionRaw, castErrorCondition)
		if !conditionMap.ok {
			return conditionMap.ReadyResult()
		}

		tpCastRes := castKey[string](conditionMap.value, "type", logReadyOrNot, castError)
		if !tpCastRes.ok {
			return tpCastRes.ReadyResult()
		}

		tp := tpCastRes.value

		valForCheck, typeFound := c.conditionsForCheck[tp]
		if !typeFound {
			continue
		}

		stat := castKey[string](conditionMap.value, "status", logReadyOrNot, castErrorCondition)

		if !stat.ok {
			return stat.ReadyResult()
		}

		statVal := stat.value
		res := statVal == valForCheck

		conditionsResults[tp] = res
	}

	conditionsLen := len(conditionsResults)

	if conditionsLen == 0 {
		returnFunc := debugLogAndReturnNotReady
		if c.readyIfNoStatusOrConditions {
			returnFunc = debugLogAndReturnReady
		}

		return returnFunc(logger, resourceName, "conditions not found")
	}

	resOfCheck := true
	falseConditions := make([]string, 0, conditionsLen)

	for conditionType, res := range conditionsResults {
		logger.LogDebugF("Condition %s for %s resource is %v\n", conditionType, resourceName, res)
		if res {
			if !c.checkAllConditions {
				return debugLogAndReturnReady(logger, resourceName, fmt.Sprintf("by condition %s", conditionType))
			}

			continue
		}

		resOfCheck = false
		falseConditions = append(falseConditions, conditionType)
	}

	if resOfCheck {
		return debugLogAndReturnReady(logger, resourceName, "by all conditions")
	}

	msg := fmt.Sprintf("next conditions not ready: %s", strings.Join(falseConditions, ","))
	return debugLogAndReturnNotReady(logger, resourceName, msg)
}
