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

package guard

import (
	"encoding/json"
	"errors"
	"os"
)

// Settings mirror the ipv4OptionsGuard section of the CNI module configuration.
type Settings struct {
	Mode                  Mode `json:"mode"`
	DisableMPTCP          bool `json:"disableMPTCP"`
	AllowKernelModuleLoad bool `json:"allowKernelModuleLoad"`
}

func DefaultSettings() Settings {
	return Settings{Mode: ModeAudit, DisableMPTCP: true}
}

// LoadSettings reads the settings file; a missing file means defaults.
func LoadSettings(path string) (Settings, error) {
	st := DefaultSettings()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return DefaultSettings(), err
	}
	if _, err := ParseMode(string(st.Mode)); err != nil {
		return DefaultSettings(), err
	}
	return st, nil
}
