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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type ResourceChecker interface {
	IsReady(ctx context.Context, resource *unstructured.Unstructured, resourceName string) (bool, error)
	// WaitAttemptsBeforeCheck
	// returns attempts without check
	// it needs for add fields by controller before checks
	WaitAttemptsBeforeCheck() int
}

type GetCheckerParams struct {
	LoggerProvider log.LoggerProvider
}

func GetCheckerByGvk(gvk *schema.GroupVersionKind, params GetCheckerParams) (ResourceChecker, error) {
	if gvk.Empty() {
		return nil, fmt.Errorf("Cannot get check by gvk: cannot be empty")
	}

	kind := gvk.Kind

	if kind == "StaticInstance" {
		return NewStaticInstanceChecker(params.LoggerProvider), nil
	}

	if _, ok := kindsWithoutCheck[kind]; ok {
		return NewExistsResourceWithoutChecker(params.LoggerProvider), nil
	}

	if phases, ok := kindsByPhases[kind]; ok {
		return NewByPhaseChecker(phases, params.LoggerProvider), nil
	}

	conditionsParams, ok := kindsByConditions[kind]

	// try to check by conditions for another but if conditions not found return ready
	// because we do not know status.conditions is available for them
	if !ok {
		conditionsChecker := NewByConditionsChecker(defaultConditions, params.LoggerProvider).
			WithWaitAttempts(3).
			WithCheckAll(false).
			WithReadyIfNoStatusOrConditions(true)

		return conditionsChecker, nil

	}

	waitAttempts := 3
	if conditionsParams.waitAttemptsBeforeCheck != nil {
		waitAttempts = *conditionsParams.waitAttemptsBeforeCheck
	}

	// Deployment and APIService NodeGroup here
	// if conditions not found wait for it
	conditionsChecker := NewByConditionsChecker(defaultConditions, params.LoggerProvider).
		WithWaitAttempts(waitAttempts).
		WithReadyIfNoStatusOrConditions(false).
		WithCheckAll(conditionsParams.checkAll)

	return conditionsChecker, nil
}

func debugLogAndReturnNotReady(logger log.Logger, resourceName, msg string) (bool, error) {
	logger.LogDebugF("Resource %s %s.\n", resourceName, msg)
	return false, nil
}

func debugLogAndReturnReady(logger log.Logger, resourceName, msg string) (bool, error) {
	logger.LogDebugF("Resource %s %s.\n", resourceName, msg)
	return true, nil
}

var kindsWithoutCheck = map[string]struct{}{
	"Namespace":                {},
	"ResourceQuota":            {},
	"LimitRange":               {},
	"PodSecurityPolicy":        {},
	"ServiceAccount":           {},
	"Secret":                   {},
	"ConfigMap":                {},
	"StorageClass":             {},
	"CustomResourceDefinition": {},
	"ClusterRole":              {},
	"ClusterRoleBinding":       {},
	"Role":                     {},
	"RoleBinding":              {},
	"Service":                  {},
	"Ingress":                  {},

	// todo huge to checkm, need to add
	"ReplicaSet":  {},
	"StatefulSet": {},
	"Job":         {},
	"CronJob":     {},
	"DaemonSet":   {},
}

var kindsByPhases = map[string]PhasesForCheck{
	"PersistentVolume": {
		"Bound":     {},
		"Available": {},
	},
	"PersistentVolumeClaim": {
		"Bound": {},
	},
	"Pod": {
		"Succeeded": {},
		"Running":   {},
	},
}

const (
	trueCondition      = "True"
	readyCondition     = "Ready"
	availableCondition = "Available"
)

var (
	defaultConditions = Conditions{
		readyCondition:     trueCondition,
		availableCondition: trueCondition,
	}
	availableConditions = Conditions{
		availableCondition: trueCondition,
	}
)

type byConditionsParams struct {
	waitAttemptsBeforeCheck *int
	conditionsForCheck      Conditions
	checkAll                bool
}

var kindsByConditions = map[string]byConditionsParams{
	"Deployment": {
		waitAttemptsBeforeCheck: pointer.Int(3),
		conditionsForCheck:      availableConditions,
		checkAll:                false,
	},
	"APIService": {
		waitAttemptsBeforeCheck: pointer.Int(2),
		conditionsForCheck:      availableConditions,
		checkAll:                false,
	},
	"NodeGroup": {
		waitAttemptsBeforeCheck: pointer.Int(5),
		conditionsForCheck: Conditions{
			readyCondition: trueCondition,
		},
		checkAll: false,
	},
}
