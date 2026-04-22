/*
Copyright 2026 Flant JSC

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

package config

import (
	"testing"
)

func TestFromBytes(t *testing.T) {
	configText :=
		`
source:
  address: localhost:5001
  ca: |
    <CA>
  user:
    name: ro
    password: ro-password
destination:
  address: localhost:5001
  ca: |
    <CA>
  user:
    name: ro
    password: ro-password
`

	cfg, err := FromBytes([]byte(configText))
	if err != nil {
		t.Errorf("cannot parse config: %v", err)
		return
	}

	err = cfg.Validate()
	if err != nil {
		t.Errorf("cannot validate config: %v", err)
	}

	t.Logf("Loaded: %#v\n", cfg)
}
