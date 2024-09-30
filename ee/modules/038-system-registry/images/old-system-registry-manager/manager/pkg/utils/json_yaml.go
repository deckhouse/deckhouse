/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"encoding/json"
	"gopkg.in/yaml.v2"
	"reflect"
)

func EqualYaml(l, r []byte) bool {
	var left, right interface{}

	if err := yaml.Unmarshal(l, &left); err != nil {
		return false
	}

	if err := yaml.Unmarshal(r, &right); err != nil {
		return false
	}

	return reflect.DeepEqual(left, right)
}

func EqualJson(l, r []byte) bool {
	var left, right interface{}

	if err := json.Unmarshal(l, &left); err != nil {
		return false
	}

	if err := json.Unmarshal(r, &right); err != nil {
		return false
	}

	return reflect.DeepEqual(left, right)
}
