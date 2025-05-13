/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package time

import (
	"encoding/json"
	"errors"
	"time"

	"sigs.k8s.io/yaml"
)

type Duration struct {
	time.Duration
}

func (d *Duration) StringPointer() *string {
	if d != nil {
		str := d.Duration.String()
		return &str
	}
	return nil
}

func (d Duration) String() string {
	return d.Duration.String()
}

// UnmarshalJSON deserializes a JSON string into Duration
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s any
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var err error
	d.Duration, err = ParseDuration(s)
	return err
}

// UnmarshalYAML deserializes a YAML string into Duration
func (d *Duration) UnmarshalYAML(value []byte) error {
	var s any
	if err := yaml.Unmarshal(value, &s); err != nil {
		return err
	}

	var err error
	d.Duration, err = ParseDuration(s)
	return err
}

func ParseDuration(value any) (time.Duration, error) {
	switch v := value.(type) {
	case float64:
		return time.Duration(v), nil
	case string:
		return time.ParseDuration(v)
	default:
		return time.Duration(0), errors.New("invalid duration")
	}
}
