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

package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

type Server struct {
	span   trace.Span
	spanMu sync.Mutex

	logger  *slog.Logger
	server  *http.Server
	started bool
	mu      sync.Mutex

	tracer     trace.Tracer
	tracerName string
}

func NewServer(
	span trace.Span,
	logger *slog.Logger,
	tracerName string,
) *Server {
	return &Server{
		span:       span,
		logger:     logger,
		tracer:     otel.Tracer(tracerName),
		tracerName: tracerName,
	}
}

func (s *Server) UpdateSpan(span trace.Span) {
	s.spanMu.Lock()
	defer s.spanMu.Unlock()

	s.span = span
}

func (s *Server) Start(_ context.Context, bindAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", s.handleTraces)

	s.server = &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	s.started = true

	select {
	case err := <-errCh:
		s.started = false
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.server == nil {
		return nil
	}

	s.started = false
	return s.server.Shutdown(ctx)
}

// OTLP JSON structs (partial, just enough for what we need)
type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}

type scopeSpans struct {
	Spans []span `json:"spans"`
}

type span struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId"`
	Name              string          `json:"name"`
	Kind              int             `json:"kind"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Attributes        []spanAttribute `json:"attributes"`
	Status            status          `json:"status"`
}

type spanAttribute struct {
	Key   string    `json:"key"`
	Value attrValue `json:"value"`
}

type attrValue struct {
	StringValue string `json:"stringValue"`
	IntValue    string `json:"intValue"`
	BoolValue   bool   `json:"boolValue"`
}

type status struct {
	Code int `json:"code"` // 0=UNSET, 1=OK, 2=ERROR
}

func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req exportTraceServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.ErrorContext(context.Background(), fmt.Sprintf("Failed to decode OTLP JSON: %v", err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				ctx := context.Background()

				traceID, err := trace.TraceIDFromHex(sp.TraceID)
				if err != nil {
					s.logger.ErrorContext(context.Background(), fmt.Sprintf("Failed to parse TraceID '%s': %v", sp.TraceID, err))
				}

				parentSpanIDToUse := trace.SpanID{}
				if sp.ParentSpanID != "" {
					parentSpanIDToUse, err = trace.SpanIDFromHex(sp.ParentSpanID)
					if err != nil {
						s.logger.ErrorContext(context.Background(), fmt.Sprintf("Failed to parse ParentSpanID '%s': %v", sp.ParentSpanID, err))
					}
				}

				if traceID.IsValid() && parentSpanIDToUse.IsValid() {
					sc := trace.NewSpanContext(trace.SpanContextConfig{
						TraceID:    traceID,
						SpanID:     parentSpanIDToUse,
						TraceFlags: trace.FlagsSampled,
						Remote:     true,
					})
					ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
				} else if s.span != nil {
					ctx = trace.ContextWithSpan(ctx, s.span)
				}

				var startTime, endTime time.Time
				// OTLP start time is string of int64 unix nano
				if sp.StartTimeUnixNano != "" {
					if nsec, err := strconv.ParseInt(sp.StartTimeUnixNano, 10, 64); err == nil {
						// Unix takes seconds and nanoseconds
						startTime = time.Unix(0, nsec)
					} else {
						s.logger.ErrorContext(context.Background(), fmt.Sprintf("Failed to parse StartTimeUnixNano '%s': %v", sp.StartTimeUnixNano, err))
					}
				}
				if sp.EndTimeUnixNano != "" {
					if nsec, err := strconv.ParseInt(sp.EndTimeUnixNano, 10, 64); err == nil {
						endTime = time.Unix(0, nsec)
					}
				}

				opts := []trace.SpanStartOption{
					trace.WithSpanKind(trace.SpanKind(sp.Kind)),
					trace.WithAttributes(semconv.ServiceName(s.tracerName)),
				}
				if !startTime.IsZero() {
					opts = append(opts, trace.WithTimestamp(startTime))
				}

				_, newSpan := s.tracer.Start(ctx, sp.Name, opts...)

				var otelAttrs []attribute.KeyValue
				if sp.SpanID != "" {
					otelAttrs = append(otelAttrs, attribute.String("external_span_id", sp.SpanID))
				}
				for _, attr := range sp.Attributes {
					switch {
					case attr.Value.StringValue != "":
						otelAttrs = append(otelAttrs, attribute.String(attr.Key, attr.Value.StringValue))
					case attr.Value.IntValue != "":
						otelAttrs = append(otelAttrs, attribute.String(attr.Key, attr.Value.IntValue))
					default:
						otelAttrs = append(otelAttrs, attribute.Bool(attr.Key, attr.Value.BoolValue))
					}
				}
				newSpan.SetAttributes(otelAttrs...)

				switch sp.Status.Code {
				case 2:
					newSpan.SetStatus(codes.Error, "")
				case 1:
					newSpan.SetStatus(codes.Ok, "")
				}

				if !endTime.IsZero() {
					newSpan.End(trace.WithTimestamp(endTime))
				} else {
					newSpan.End()
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
