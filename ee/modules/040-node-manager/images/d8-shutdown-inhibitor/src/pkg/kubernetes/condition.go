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
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func ConditionFromJSON(condition []byte) (*Condition, error) {
	if len(condition) == 0 {
		return nil, nil
	}

	if condition[0] != '{' {
		return nil, fmt.Errorf("expect condition as JSON object, got: %s", string(condition))
	}

	var conditionJSON Condition
	err := json.Unmarshal(condition, &conditionJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal condition JSON object: %w (%s)", err, string(condition))
	}
	return &conditionJSON, nil
}
