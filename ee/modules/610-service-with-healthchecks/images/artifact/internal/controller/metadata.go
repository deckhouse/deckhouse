/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	// Sorted lists of the keys copied from the parent on the previous reconciliation.
	// They let us drop a key that disappeared from the parent and keep keys owned by
	// other controllers (MetalLB, cloud providers). Both lists live in annotations,
	// because a label value can not contain a comma.
	propagatedAnnotationsKey = "network.deckhouse.io/propagated-annotations"
	propagatedLabelsKey      = "network.deckhouse.io/propagated-labels"
)

// Annotations describing the parent object itself, never copied to the child Service.
var nonPropagatedAnnotations = map[string]struct{}{
	corev1.LastAppliedConfigAnnotation: {},
	"meta.helm.sh/release-name":        {},
	"meta.helm.sh/release-namespace":   {},
	propagatedAnnotationsKey:           {},
	propagatedLabelsKey:                {},
}

func propagatedAnnotations(annotations map[string]string) map[string]string {
	result := make(map[string]string, len(annotations))
	for key, value := range annotations {
		if _, skip := nonPropagatedAnnotations[key]; skip {
			continue
		}
		result[key] = value
	}
	return result
}

// mergePropagated adds the desired keys to the current ones and removes the keys
// that were propagated earlier but are gone from the parent now.
func mergePropagated(current, desired map[string]string, previouslyPropagated string) map[string]string {
	result := make(map[string]string, len(current)+len(desired))
	for key, value := range current {
		result[key] = value
	}

	for _, key := range splitKeys(previouslyPropagated) {
		if _, stillDesired := desired[key]; !stillDesired {
			delete(result, key)
		}
	}

	for key, value := range desired {
		result[key] = value
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// setPropagatedKeys writes the tracking annotation, removing it when nothing is propagated.
func setPropagatedKeys(annotations map[string]string, trackingKey string, propagated map[string]string) map[string]string {
	joined := joinKeys(propagated)
	if joined == "" {
		delete(annotations, trackingKey)
		return annotations
	}
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	annotations[trackingKey] = joined
	return annotations
}

func joinKeys(source map[string]string) string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func splitKeys(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ",")
}

func isSubset(desired, actual map[string]string) bool {
	for key, value := range desired {
		if actualValue, found := actual[key]; !found || actualValue != value {
			return false
		}
	}
	return true
}
