/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubeapi

import (
	"encoding/json"
	"fmt"
)

type ActionStatus struct {
	Name      string `json:"name"`
	Priority  int    `json:"priority"`
	Approved  bool   `json:"approved"`
	Completed bool   `json:"completed"`
}

func (actSt *ActionStatus) toString() (string, error) {
	jsonData, err := json.Marshal(actSt)
	if err != nil {
		return "", fmt.Errorf("json serialization error: %v", err)
	}
	return string(jsonData), nil
}

func fromStringToStat(strActSt string) (ActionStatus, error) {
	actSt := ActionStatus{}
	err := json.Unmarshal([]byte(strActSt), &actSt)
	if err != nil {
		return actSt, fmt.Errorf("json decoding error: %v", err)
	}
	return actSt, nil
}

func ActionStatusEqual(l, r *ActionStatus) bool {
	if l == nil || r == nil {
		return false
	}
	return l.Name == r.Name &&
		l.Priority == r.Priority &&
		l.Approved == r.Approved
}
