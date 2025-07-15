/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package kubernetes

import (
	"encoding/json"
	"fmt"
)

type Condition struct {
	Reason string `json:"reason"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

func ConditionFromJSON(condition []byte) (*Condition, error) {
	var conditionJSON Condition
	if string(condition) != "''" {
		err := json.Unmarshal(condition, &conditionJSON)
		if err != nil {
			return nil, fmt.Errorf("condition json unmarshal: %w", err)
		}
	}

	return &conditionJSON, nil
}
