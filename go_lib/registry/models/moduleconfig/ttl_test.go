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

package moduleconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTTL(t *testing.T) {
	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		ttl    string
		output output
	}{
		// Valid cases
		{
			name: "valid settings with ttl 5m",
			ttl:  "5m",
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with ttl 123m123s",
			ttl:  "123m123s",
			output: output{
				err: false,
			},
		},
		{
			name: "valid empty ttl",
			ttl:  "",
			output: output{
				err: false,
			},
		},

		// Invalid cases
		{
			name: "invalid TTL regexp",
			ttl:  "invalid-ttl",
			output: output{
				err:    true,
				errMsg: "does not match required pattern",
			},
		},
		{
			name: "invalid TTL duration",
			ttl:  "4m59s",
			output: output{
				err:    true,
				errMsg: "must be at least",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTTL(tt.ttl)

			if tt.output.err {
				require.Error(t, err)
				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
