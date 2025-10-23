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
