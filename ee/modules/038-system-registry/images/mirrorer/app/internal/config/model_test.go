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

package config

import (
	"strings"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	configText :=
		`
ca: /system_registry_pki/ca.crt
sleep: 10
users:
  puller:
    name: puller-user
    password: puller-password
  pusher:
    name: pusher-user
    password: pusher_password

local: "localhost:5001"
remote:
  - "test:5001"

`

	cfg, err := parse(strings.NewReader(configText))
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
