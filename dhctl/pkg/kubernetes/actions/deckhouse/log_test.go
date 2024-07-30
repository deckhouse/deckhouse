/*
Copyright 2022 Flant JSC

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

package deckhouse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_is_error_line(t *testing.T) {
	tests := []struct {
		name    string
		logLine string
		result  bool
	}{
		// Positive cases.
		{
			"level=error",
			`{"level":"error","msg":"Global hook failed, requeue task to retry after delay ... ","binding":"onStartup","event.type":"OperatorStartup","hook":"module-config/startup_sync.go","hook.type":"global","queue":"main","task.id":"6d685def-ad8e-4961-a14b-a422e1f76a31","time":"2022-06-17T08:51:32Z"}`,
			true,
		},
		{
			"stderr",
			`{"level":"info","output":"stderr","binding":"kubernetes","binding.name":"secrets","event.type":"OperatorStartup","hook":"300-prometheus/hooks/additional_configs_render","hook.type":"module","module":"prometheus","msg":"secret/prometheus-main-additional-configs unchanged","queue":"/modules/prometheus","task.id":"ba36269b-6699-42a0-9337-dce59b48f68b","time":"2022-06-17T08:53:27Z"}`,
			true,
		},
		// Negative cases.
		{
			"level=info",
			`{"level":"info","msg":"Module hook start documentation/810-documentation/hooks/https/copy_custom_certificate.go","binding":"beforeHelm","event.type":"OperatorStartup","hook":"810-documentation/hooks/https/copy_custom_certificate.go","hook.type":"module","module":"documentation","queue":"main","task.id":"e7f06e0c-2304-421a-a856-2684f5cd4914","time":"2022-06-17T08:54:22Z"}`,
			false,
		},
		{
			"stdout",
			`{"level":"info","output":"stdout","binding":"kubernetes","binding.name":"secrets","event.type":"OperatorStartup","hook":"300-prometheus/hooks/additional_configs_render","hook.type":"module","module":"prometheus","msg":"secret/prometheus-main-additional-configs unchanged","queue":"/modules/prometheus","task.id":"ba36269b-6699-42a0-9337-dce59b48f68b","time":"2022-06-17T08:53:27Z"}`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			var lines int
			parseLogByLine([]byte(tt.logLine), func(line *logLine) bool {
				result = isErrorLine(line)
				lines++
				// Stop on first line.
				return false
			})
			require.Equal(t, 1, lines, "Should parse log line")
			if tt.result {
				require.True(t, result, "Should detect error message")
			} else {
				require.False(t, result, "Should not detect error message")
			}
		})
	}
}
