/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
sleep: 10s
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
