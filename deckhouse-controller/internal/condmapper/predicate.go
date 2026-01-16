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

// Match holds predicate result with source condition for Reason/Message.
type Match struct {
	Ok     bool   // predicate matched
	Source string // internal condition that caused match
}

// Pred evaluates state and returns match with source.
type Pred func(State) Match

// IsTrue checks if a single condition is True.
func IsTrue(cond string) Pred {
	return func(s State) Match {
		c, ok := s.Internal[cond]
		if ok && c.Status == metav1.ConditionTrue {
			return Match{Ok: true, Source: cond}
		}
		return Match{}
	}
}

// AllTrue checks if all conditions are True. Returns first condition as source.
func AllTrue(conds ...string) Pred {
	return func(s State) Match {
		for _, cond := range conds {
			c, ok := s.Internal[cond]
			if !ok || c.Status != metav1.ConditionTrue {
				return Match{}
			}
		}
		if len(conds) > 0 {
			return Match{Ok: true, Source: conds[0]}
		}
		return Match{Ok: true}
	}
}

// AnyFalse checks if any condition is False. Returns failing condition as source.
func AnyFalse(conds ...string) Pred {
	return func(s State) Match {
		for _, cond := range conds {
			c, ok := s.Internal[cond]
			if ok && c.Status == metav1.ConditionFalse {
				return Match{Ok: true, Source: cond}
			}
		}
		return Match{}
	}
}

// And combines predicates with logical AND. Returns last source.
func And(preds ...Pred) Pred {
	return func(s State) Match {
		var last Match
		for _, p := range preds {
			m := p(s)
			if !m.Ok {
				return Match{}
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
func Or(preds ...Pred) Pred {
	return func(s State) Match {
		for _, p := range preds {
			if m := p(s); m.Ok {
				return m
			}
		}
		return Match{}
	}
}

// Not negates a predicate. Clears source on negation.
func Not(p Pred) Pred {
	return func(s State) Match {
		if m := p(s); !m.Ok {
			return Match{Ok: true}
		}
		return Match{}
	}
}

// VersionChanged checks if version changed flag is set.
func VersionChanged() Pred {
	return func(s State) Match {
		if s.VersionChanged {
			return Match{Ok: true}
		}
		return Match{}
	}
}

// ExtTrue checks if an external condition is True.
func ExtTrue(cond string) Pred {
	return func(s State) Match {
		c, ok := s.External[cond]
		if ok && c.Status == metav1.ConditionTrue {
			return Match{Ok: true}
		}
		return Match{}
	}
}
