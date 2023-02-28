/*
Copyright 2023 Flant JSC

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

package scheduler

import (
	"testing"
)

func Test_stsSelectorByStorageClass_Select(t *testing.T) {
	defaultStorageClass := "default"

	tests := []struct {
		name   string
		input  func() (State, string)
		assert func(*testing.T, string, error)
	}{
		{
			name: "filled state and used current storage class; none to change",
			input: func() (State, string) {
				return fakeState(), defaultStorageClass
			},
			assert: assertNone,
		},
		{
			name: "filled state and unused default storage class; selects any to deploy",
			input: func() (State, string) {
				return fakeState(), "newer"
			},
			assert: assertAny,
		},
		{
			name: "selects index with deviating storageclass",
			input: func() (State, string) {
				state := fakeState()
				state["d"].StorageClass = "outdated"
				return state, defaultStorageClass
			},
			assert: assertOk("d"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, sc := tt.input()
			s := &selectByStorageClass{
				storageClass: sc,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}
