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

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

type ResourceChecker interface {
	IsReady(ctx context.Context, resource *unstructured.Unstructured, resourceName string) (bool, error)
	// WaitAttemptsBeforeCheck
	// returns attempts without check
	// it needs for add fields by controller before checks
	WaitAttemptsBeforeCheck() int
}

type GetCheckerParams struct {
}

func GetCheckerByGvk(gvk *schema.GroupVersionKind, _ GetCheckerParams) (ResourceChecker, error) {
	if gvk.Empty() {
		return nil, fmt.Errorf("Cannot get checker by gvk: gvk cannot be empty")
	}

	kind := gvk.Kind

	if kind == "StaticInstance" {
		return NewStaticInstanceChecker(), nil
	}

	if _, ok := kindsWithoutCheck[kind]; ok {
		return NewExistsResourceWithoutChecker(), nil
	}

	if phases, ok := kindsByPhases[kind]; ok {
		return NewByPhaseChecker(phases), nil
	}

	conditionsParams, ok := kindsByConditions[kind]

	// try to check by conditions for another but if conditions not found return ready
	// because we do not know status.conditions is available for them
	if !ok {
		conditionsChecker := NewByConditionsChecker(defaultConditions).
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
	conditionsChecker := NewByConditionsChecker(defaultConditions).
		WithWaitAttempts(waitAttempts).
		WithReadyIfNoStatusOrConditions(false).
		WithCheckAll(conditionsParams.checkAll)

	return conditionsChecker, nil
}

func debugLogAndReturnNotReady(ctx context.Context, resourceName, msg string) (bool, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Resource %s %s.", resourceName, msg))
	return false, nil
}

func debugLogAndReturnReady(ctx context.Context, resourceName, msg string) (bool, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Resource %s %s.", resourceName, msg))
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
		waitAttemptsBeforeCheck: new(3),
		conditionsForCheck:      availableConditions,
		checkAll:                false,
	},
	"APIService": {
		waitAttemptsBeforeCheck: new(2),
		conditionsForCheck:      availableConditions,
		checkAll:                false,
	},
	"NodeGroup": {
		waitAttemptsBeforeCheck: new(5),
		conditionsForCheck: Conditions{
			readyCondition: trueCondition,
		},
		checkAll: false,
	},
}
