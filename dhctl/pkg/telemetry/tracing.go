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

package telemetry

import (
	"context"
	"runtime"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	ottrace "go.opentelemetry.io/otel/trace"
)

var (
	mu     sync.Mutex
	tracer ottrace.Tracer
)

func Tracer() ottrace.Tracer {
	if tracer == nil {
		mu.Lock()
		tracer = otel.Tracer(traceApplicationName)
		mu.Unlock()
	}

	return tracer
}

func StartSpan(ctx context.Context, name string, opts ...ottrace.SpanStartOption) (context.Context, ottrace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

func traceName() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}

	// It contains full path of pkg, caller and func
	details := runtime.FuncForPC(pc)

	functionFullNameS := strings.Split(details.Name(), "/")

	functionFullName := functionFullNameS[len(functionFullNameS)-1]

	functionFullName = strings.ReplaceAll(functionFullName, ".", "/")
	functionFullName = strings.ReplaceAll(functionFullName, "(", "")
	functionFullName = strings.ReplaceAll(functionFullName, ")", "")
	functionFullName = strings.ReplaceAll(functionFullName, "*", "")

	return functionFullName
}
