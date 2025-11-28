// Copyright 2025 Flant JSC
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

package helpers

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSplitAddressAndPath(t *testing.T) {
	type output = struct {
		address string
		path    string
	}

	tests := []struct {
		name   string
		input  string
		output output
	}{
		{
			name:  "only address without path",
			input: "example.com",
			output: output{
				address: "example.com",
				path:    "",
			},
		},
		{
			name:  "address with single path",
			input: "example.com/path",
			output: output{
				address: "example.com",
				path:    "/path",
			},
		},
		{
			name:  "address with trailing slash",
			input: "example.com/",
			output: output{
				address: "example.com",
				path:    "",
			},
		},
		{
			name:  "address with nested path",
			input: "example.com/path/to/resource",
			output: output{
				address: "example.com",
				path:    "/path/to/resource",
			},
		},
		{
			name:  "address with port and path",
			input: "example.com:8080/api/v1",
			output: output{
				address: "example.com:8080",
				path:    "/api/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, path := SplitAddressAndPath(tt.input)
			assert.Equal(t, tt.output.address, address)
			assert.Equal(t, tt.output.path, path)
		})
	}
}
