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

package condmapper

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// match holds predicate result with source condition for Reason/Message.
type match struct {
	Ok     bool   // predicate matched
	Source string // internal condition that caused match
}

// Predicate evaluates state and returns match with source.
type Predicate func(State) match

// IsTrue checks if a single condition is True.
func IsTrue(cond string) Predicate {
	return func(s State) match {
		c, ok := s.Internal[cond]
		if ok && c.Status == metav1.ConditionTrue {
			return match{Ok: true, Source: cond}
		}
		return match{}
	}
}

// AllTrue checks if all conditions are True. Returns first condition as source.
func AllTrue(conds ...string) Predicate {
	return func(s State) match {
		for _, cond := range conds {
			c, ok := s.Internal[cond]
			if !ok || c.Status != metav1.ConditionTrue {
				return match{}
			}
		}
		if len(conds) > 0 {
			return match{Ok: true, Source: conds[0]}
		}
		return match{Ok: true}
	}
}

// AnyFalse checks if any condition is False. Returns failing condition as source.
func AnyFalse(conds ...string) Predicate {
	return func(s State) match {
		for _, cond := range conds {
			c, ok := s.Internal[cond]
			if ok && c.Status == metav1.ConditionFalse {
				return match{Ok: true, Source: cond}
			}
		}
		return match{}
	}
}

// And combines predicates with logical AND. Returns last source.
func And(preds ...Predicate) Predicate {
	return func(s State) match {
		var last match
		for _, p := range preds {
			m := p(s)
			if !m.Ok {
				return match{}
			}
			if m.Source != "" {
				last = m
			}
		}
		last.Ok = true
		return last
	}
}

// Or combines predicates with logical OR. Returns first matching source.
func Or(preds ...Predicate) Predicate {
	return func(s State) match {
		for _, p := range preds {
			if m := p(s); m.Ok {
				return m
			}
		}
		return match{}
	}
}

// Not negates a predicate. Clears source on negation.
func Not(p Predicate) Predicate {
	return func(s State) match {
		if m := p(s); !m.Ok {
			return match{Ok: true}
		}
		return match{}
	}
}

// VersionChanged checks if version changed flag is set.
func VersionChanged() Predicate {
	return func(s State) match {
		if s.VersionChanged {
			return match{Ok: true}
		}
		return match{}
	}
}

// ExtTrue checks if an external condition is True.
func ExtTrue(cond string) Predicate {
	return func(s State) match {
		c, ok := s.External[cond]
		if ok && c.Status == metav1.ConditionTrue {
			return match{Ok: true}
		}
		return match{}
	}
}

// ExtPresent checks if an external condition exists (regardless of status).
func ExtPresent(cond string) Predicate {
	return func(s State) match {
		if _, ok := s.External[cond]; ok {
			return match{Ok: true}
		}
		return match{}
	}
}
