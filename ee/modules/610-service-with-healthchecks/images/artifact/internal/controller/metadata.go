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

	// heritageLabelKey marks the child Service as belonging to Deckhouse, the same way the rest of
	// the platform labels its own resources. Its value is fixed by the module, so it is owned by
	// the controller rather than copied from the parent; see setManagedLabels.
	heritageLabelKey   = "heritage"
	heritageLabelValue = "deckhouse"
)

// Annotations describing the parent object itself, never copied to the child Service.
var nonPropagatedAnnotations = map[string]struct{}{
	corev1.LastAppliedConfigAnnotation: {},
	"meta.helm.sh/release-name":        {},
	"meta.helm.sh/release-namespace":   {},
	propagatedAnnotationsKey:           {},
	propagatedLabelsKey:                {},
}

// Labels the controller owns on the child Service and never copies from the parent: their value is
// fixed by the module, so a parent label of the same key is neither propagated nor able to override
// them. Keeping them out of propagation also keeps them out of propagatedLabelsKey, so they are not
// removed when the parent lacks them.
var nonPropagatedLabels = map[string]struct{}{
	heritageLabelKey: {},
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

func propagatedLabels(labels map[string]string) map[string]string {
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		if _, skip := nonPropagatedLabels[key]; skip {
			continue
		}
		result[key] = value
	}
	return result
}

// setManagedLabels sets the labels the controller always puts on the child Service, regardless of
// the parent. Applied after propagation, so they win over any parent label of the same key; excluded
// from propagatedLabels, so they are not tracked as copied-from-parent keys.
func setManagedLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[heritageLabelKey] = heritageLabelValue
	return labels
}

// hasManagedLabels reports whether the child Service already carries the controller-owned labels, so
// a Service created before they were introduced is not mistaken for one that is already in sync.
func hasManagedLabels(labels map[string]string) bool {
	return labels[heritageLabelKey] == heritageLabelValue
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
