package deckhouse

import (
	"github.com/stretchr/testify/require"
	"testing"
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
			`{"level":"info","msg":"Module hook start deckhouse-web/810-deckhouse-web/hooks/https/copy_custom_certificate.go","binding":"beforeHelm","event.type":"OperatorStartup","hook":"810-deckhouse-web/hooks/https/copy_custom_certificate.go","hook.type":"module","module":"deckhouse-web","queue":"main","task.id":"e7f06e0c-2304-421a-a856-2684f5cd4914","time":"2022-06-17T08:54:22Z"}`,
			false,
		},
		{
			"stdout",
			`{"level":"info","output":"stdout","binding":"kubernetes","binding.name":"secrets","event.type":"OperatorStartup","hook":"300-prometheus/hooks/additional_configs_render","hook.type":"module","module":"prometheus","msg":"secret/prometheus-main-additional-configs unchanged","queue":"/modules/prometheus","task.id":"ba36269b-6699-42a0-9337-dce59b48f68b","time":"2022-06-17T08:53:27Z"}`,
			false,
		},
		{
			"stderr from tiller",
			`{"operator.component":"tiller","msg":"some error message","level":"info","output":"stderr","time":"2022-06-17T08:53:27Z"}`,
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
