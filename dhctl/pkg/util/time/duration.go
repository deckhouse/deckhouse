// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package time

import (
	"errors"
	"encoding/json"
	"time"

	"sigs.k8s.io/yaml"
)

type Duration struct {
	time.Duration
}

func (d Duration) String() string {
	return d.Duration.String()
}

// UnmarshalJSON deserializes a JSON string into Duration
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s interface{}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var err error
	d.Duration, err = ParseDuration(s)
	return err
}

// UnmarshalYAML deserializes a YAML string into Duration
func (d *Duration) UnmarshalYAML(value []byte) error {
	var s interface{}
	if err := yaml.Unmarshal(value, &s); err != nil {
		return err
	}

	var err error
	d.Duration, err = ParseDuration(s)
	return err
}

func ParseDuration(value interface{}) (time.Duration, error) {
	switch v := value.(type) {
	case float64:
		return time.Duration(v), nil
	case string:
		return time.ParseDuration(v)
	default:
		return time.Duration(0), errors.New("invalid duration")
	}
}
