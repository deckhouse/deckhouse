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

package operation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
)

// tracingSuite records the spans of one operation per test through the global tracer provider.
type tracingSuite struct {
	suite.Suite

	recorder *tracetest.SpanRecorder
	previous trace.TracerProvider
}

func TestTracing(t *testing.T) {
	suite.Run(t, new(tracingSuite))
}

func (s *tracingSuite) SetupTest() {
	s.recorder = tracetest.NewSpanRecorder()
	s.previous = otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(s.recorder)))
}

func (s *tracingSuite) TearDownTest() {
	otel.SetTracerProvider(s.previous)
}

// run starts the operation and waits for its result.
func (s *tracingSuite) run(ctx context.Context, steps ...operation.Step) operation.Result {
	return <-operation.Start(ctx, "echo", operation.Run, "inputs", steps).Done()
}

// spans returns the ended spans by name.
func (s *tracingSuite) spans() map[string]sdktrace.ReadOnlySpan {
	out := make(map[string]sdktrace.ReadOnlySpan)
	for _, span := range s.recorder.Ended() {
		out[span.Name()] = span
	}

	return out
}

func ok(name string) operation.Step {
	return operation.Step{Name: name, Run: func(context.Context) error { return nil }}
}

// TestStepsAreChildrenOfTheOperation: the operation span carries the package, kind and key, and
// every step is a child span of it.
func (s *tracingSuite) TestStepsAreChildrenOfTheOperation() {
	res := s.run(context.Background(), ok("render"), ok("upgrade"))
	s.Require().Nil(res.Failed)

	spans := s.spans()
	s.Require().Len(spans, 3)

	op := spans[string(operation.Run)]
	s.Require().NotNil(op)
	s.ElementsMatch([]attribute.KeyValue{
		attribute.String("package", "echo"),
		attribute.String("kind", string(operation.Run)),
		attribute.String("key", "inputs"),
	}, op.Attributes())
	s.Equal(codes.Unset, op.Status().Code)

	for _, name := range []string{"render", "upgrade"} {
		s.Equal(op.SpanContext().SpanID(), spans[name].Parent().SpanID(), name)
	}
}

// TestFailedStepMarksTheOperation: the failing step and the operation are errors, and the steps
// after it get no span.
func (s *tracingSuite) TestFailedStepMarksTheOperation() {
	failing := operation.Step{Name: "upgrade", Run: func(context.Context) error { return errors.New("boom") }}

	res := s.run(context.Background(), ok("render"), failing, ok("monitor"))
	s.Require().NotNil(res.Failed)

	spans := s.spans()
	s.Require().Len(spans, 3)
	s.NotContains(spans, "monitor")

	step := spans["upgrade"]
	s.Equal(codes.Error, step.Status().Code)
	s.Len(step.Events(), 1, "the error is recorded")

	op := spans[string(operation.Run)]
	s.Equal(codes.Error, op.Status().Code)
	s.Contains(op.Attributes(), attribute.String("failed_step", "upgrade"))
	s.Contains(op.Attributes(), attribute.Bool("canceled", false))
}

// TestCanceledOperationIsMarked: a step that ends on the owner's cancellation marks the operation
// span canceled.
func (s *tracingSuite) TestCanceledOperationIsMarked() {
	started := make(chan struct{})
	blocking := operation.Step{Name: "wait", Run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()

		return ctx.Err()
	}}

	op := operation.Start(context.Background(), "echo", operation.Run, "inputs", []operation.Step{blocking})
	<-started
	op.Cancel()

	res := <-op.Done()
	s.Require().True(res.Canceled)

	s.Contains(s.spans()[string(operation.Run)].Attributes(), attribute.Bool("canceled", true))
}
