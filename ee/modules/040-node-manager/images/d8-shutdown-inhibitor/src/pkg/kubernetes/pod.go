/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"encoding/json"
	"fmt"
)

type PodList struct {
	Items []Pod `json:"items"`
}

type Pod struct {
	Metadata *PodMetadata           `json:"metadata"`
	Status   *PodStatus             `json:"status"`
	Others   map[string]interface{} `json:"-"`
}

type PodMetadata struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Others      map[string]interface{} `json:"-"`
}

type PodStatus struct {
	Phase  string                 `json:"phase"`
	Others map[string]interface{} `json:"-"`
}

func podsListFromJSON(podsJson []byte) (*PodList, error) {
	var podList PodList

	err := json.Unmarshal(podsJson, &podList)
	if err != nil {
		return nil, fmt.Errorf("pods list json unmarshal: %w", err)
	}

	return &podList, nil
}
