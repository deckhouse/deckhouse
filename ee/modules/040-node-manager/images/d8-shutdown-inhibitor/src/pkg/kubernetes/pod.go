/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type Pod = corev1.Pod
type PodList = corev1.PodList

func podsFromJSON(podsJSON []byte) (*corev1.PodList, error) {
	var podList corev1.PodList
	if err := json.Unmarshal(podsJSON, &podList); err != nil {
		return nil, fmt.Errorf("pods list json unmarshal: %w", err)
	}
	return &podList, nil
}
