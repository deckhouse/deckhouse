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

package nodeservices

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProxyConfig_Validate(t *testing.T) {
	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		input  ProxyConfig
		output output
	}{
		// Valid cases
		{
			name: "all fields empty - valid",
			input: ProxyConfig{
				HTTP:    "",
				HTTPS:   "",
				NoProxy: "",
			},
			output: output{
				err: false,
			},
		},
		{
			name: "all fields no empty - valid",
			input: ProxyConfig{
				HTTP:    "http://proxy.example.com:8080",
				HTTPS:   "http://proxy.example.com:8443",
				NoProxy: "localhost,127.0.0.1,.internal",
			},
			output: output{
				err: false,
			},
		},
		{
			name: "only HTTP proxy - valid",
			input: ProxyConfig{
				HTTP:    "http://proxy.example.com:8080",
				HTTPS:   "",
				NoProxy: "",
			},
			output: output{
				err: false,
			},
		},
		{
			name: "only HTTPS proxy - valid",
			input: ProxyConfig{
				HTTP:    "",
				HTTPS:   "http://proxy.example.com:8443",
				NoProxy: "",
			},
			output: output{
				err: false,
			},
		},
		{
			name: "HTTP and HTTPS proxy - valid",
			input: ProxyConfig{
				HTTP:    "http://proxy.example.com:8080",
				HTTPS:   "http://proxy.example.com:8443",
				NoProxy: "",
			},
			output: output{
				err: false,
			},
		},
		// Invalid cases
		{
			name: "empty config with NoProxy - invalid",
			input: ProxyConfig{
				HTTP:    "",
				HTTPS:   "",
				NoProxy: "localhost,127.0.0.1",
			},
			output: output{
				err:    true,
				errMsg: "NoProxy should be empty when HTTP and HTTPS are empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

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
