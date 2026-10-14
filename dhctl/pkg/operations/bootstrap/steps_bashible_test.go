// Copyright 2026 Flant JSC
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

package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestABootstrapScriptTimingLineBecomesASpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	require.NoError(t, recordTimingSpan(context.Background(), "[bashible-timing] step=nodelet_install section=package dur=2.500s\n"))
	require.NoError(t, recordTimingSpan(context.Background(), "Current IPv4 addresses: 10.0.0.1"))
	require.ErrorContains(t, recordTimingSpan(context.Background(), "[bashible-timing] step=nodelet_install section=start dur=1.2.3s"), "parse the duration")

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "nodelet_install.package", spans[0].Name())
	require.Equal(t, 2500*time.Millisecond, spans[0].EndTime().Sub(spans[0].StartTime()))
}
