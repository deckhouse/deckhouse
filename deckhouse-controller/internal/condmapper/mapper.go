package condmapper

import (
	"maps"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// State holds internal and external conditions for mapping.
type State struct {
	VersionChanged bool                        // true if package version changed
	Internal       map[string]metav1.Condition // source conditions
	External       map[string]metav1.Condition // previous external state
}

// Mapper applies rules to compute external conditions.
type Mapper struct {
	Rules []Rule
}

// Map evaluates all rules and returns computed external conditions.
func (m Mapper) Map(state State) []metav1.Condition {
	result := make(map[string]metav1.Condition, len(m.Rules))

	for _, r := range m.Rules {
		if r.OnlyIf != nil && !r.OnlyIf(state).Ok {
			continue
		}

		// Sticky: skip if already True in previous state (no update needed)
		if r.Sticky {
			if c, ok := state.External[r.Type]; ok && c.Status == metav1.ConditionTrue {
				continue
			}
		}

		// FalseIf checked first: failure state takes precedence
		var match Match
		var status metav1.ConditionStatus

		if r.FalseIf != nil {
			match = r.FalseIf(state)
			if match.Ok {
				status = metav1.ConditionFalse
			}
		}
		if !match.Ok && r.TrueIf != nil {
			match = r.TrueIf(state)
			if match.Ok {
				status = metav1.ConditionTrue
			}
		}

		if !match.Ok {
			if c, ok := state.External[r.Type]; ok {
				result[r.Type] = c
			}
			continue
		}

		// build condition with Reason/Message from source
		cond := metav1.Condition{
			Type:   r.Type,
			Status: status,
		}
		if src, ok := state.Internal[match.Source]; ok {
			cond.Reason = src.Reason
			cond.Message = src.Message
		}
		result[r.Type] = cond
	}

	// Convert map to slice
	return slices.Collect(maps.Values(result))
}
