/*
Copyright 2024 Flant JSC

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

package updater

import (
	"fmt"
)

var _UpdateModeMap = map[UpdateMode]string{
	ModeAutoPatch: string(ModeAutoPatch),
	ModeAuto:      string(ModeAuto),
	ModeManual:    string(ModeManual),
}

// String implements the Stringer interface.
func (x UpdateMode) String() string {
	if str, ok := _UpdateModeMap[x]; ok {
		return str
	}
	return fmt.Sprintf("UpdateMode(%s)", string(x))
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x UpdateMode) IsValid() bool {
	_, ok := _UpdateModeMap[x]
	return ok
}

var _UpdateModeValue = map[string]UpdateMode{
	string(ModeAutoPatch): ModeAutoPatch,
	string(ModeAuto):      ModeAuto,
	string(ModeManual):    ModeManual,
}

// ParseUpdateMode attempts to convert a string to a UpdateMode.
//
// AutoPatch used by default
func ParseUpdateMode(name string) UpdateMode {
	if x, ok := _UpdateModeValue[name]; ok {
		return x
	}

	return ModeAutoPatch
}
