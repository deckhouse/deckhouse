/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
