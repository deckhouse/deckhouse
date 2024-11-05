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

package flags

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		args            []string
		expectedVerbose bool
		wantErr         bool
	}{
		{
			nil,
			false,
			false,
		},
		{
			[]string{"-test.run", `^\QTestReleaseControllerTestSuite\E$/^\QTestCreateReconcile\E$/^\QAutoPatch\E$/^\Qpatch_update_respect_window\E$`},
			false,
			false,
		},
		{
			[]string{"-test.v", "-test.run", `some-invalid-regexp\`},
			true,
			true,
		},
		{
			[]string{"-not.defined", "-test.v"},
			false, // TODO: true
			false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Parse(%s)", tt.args), func(t *testing.T) {
			if err := Parse(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.expectedVerbose != Verbose {
				t.Fail()
			}
		})
	}
}
