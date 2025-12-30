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

package conditionmapper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Predicate is a function that evaluates a Status and returns true/false.
// Used to control when rules should apply.
type Predicate func(status Status) bool

// And returns a predicate that is true only if ALL given predicates are true.
func And(predicates ...Predicate) Predicate {
	return func(status Status) bool {
		for _, predicate := range predicates {
			if !predicate(status) {
				return false
			}
		}
		return true
	}
}

// Or returns a predicate that is true if ANY given predicate is true.
func Or(predicates ...Predicate) Predicate {
	return func(status Status) bool {
		for _, predicate := range predicates {
			if predicate(status) {
				return true
			}
		}
		return false
	}
}

// Not returns a predicate that inverts the given predicate.
func Not(predicate Predicate) Predicate {
	return func(status Status) bool {
		return !predicate(status)
	}
}

// allInternalTrue returns a predicate that checks if all specified internal conditions are True.
// Missing conditions are ignored (treated as if they don't affect the result).
// Returns true only if ALL present conditions are True.
func allInternalTrue(conditions ...string) Predicate {
	return func(status Status) bool {
		for _, cond := range conditions {
			internal, ok := status.Internal[cond]
			if !ok {
				continue // Skip missing conditions
			}

			if internal.Status != metav1.ConditionTrue {
				return false
			}
		}
		return true
	}
}

// anyInternalFalse returns a predicate that checks if any specified internal condition is False.
// Missing conditions are ignored.
// Returns true if ANY present condition is explicitly False.
func anyInternalFalse(conditions ...string) Predicate {
	return func(status Status) bool {
		for _, cond := range conditions {
			internal, ok := status.Internal[cond]
			if !ok {
				continue // Skip missing conditions
			}

			if internal.Status == metav1.ConditionFalse {
				return true
			}
		}
		return false
	}
}

// VersionChanged returns a predicate that checks if the version changed flag matches the expected value.
// Used to trigger different rules for initial installation vs. updates.
func VersionChanged(changed bool) Predicate {
	return func(status Status) bool {
		return status.VersionChanged == changed
	}
}

// ExternalNotTrue returns a predicate that checks if an external condition is not True.
// Returns true if the condition is missing, False, or Unknown.
func ExternalNotTrue(cond string) Predicate {
	return func(status Status) bool {
		external, ok := status.External[cond]
		if !ok {
			return true // Missing is considered "not true"
		}
		return external.Status != metav1.ConditionTrue
	}
}

// ExternalTrue returns a predicate that checks if an external condition is True.
// Returns false if the condition is missing, False, or Unknown.
func ExternalTrue(cond string) Predicate {
	return func(status Status) bool {
		external, ok := status.External[cond]
		if !ok {
			return false // Missing is not true
		}
		return external.Status == metav1.ConditionTrue
	}
}
