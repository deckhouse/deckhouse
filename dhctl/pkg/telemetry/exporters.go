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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/090809/oteljsonl"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

type exporterType string

const (
	httpExporter exporterType = "http"
	grpcExporter exporterType = "grpc"
)

func configureRemoteExporter(ctx context.Context) (sdktrace.SpanExporter, sdkmetric.Exporter, sdklog.Exporter, error) {
	protocol, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_PROTOCOL")
	if !ok || protocol == "" {
		protocol = "http/json"
	}

	var usedExporter = httpExporter
	if strings.Contains(protocol, "grpc") {
		usedExporter = grpcExporter
	}

	exporterEndpointAuthorization := os.Getenv("OTEL_EXPORTER_OTLP_AUTHORIZATION")
	if exporterEndpointAuthorization != "" {
		exporterEndpointAuthorization = "Bearer " + exporterEndpointAuthorization
	}

	traceExporter, err := CreateTraceRemoteExporter(ctx, usedExporter, exporterEndpointAuthorization)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create OTLP Trace exporter: %w", err)
	}

	metricExporter, err := CreateMetricRemoteExporter(ctx, usedExporter, exporterEndpointAuthorization)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create OTLP Metric exporter: %w", err)
	}

	logsExporter, err := CreateLogsRemoteExporter(ctx, usedExporter, exporterEndpointAuthorization)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create OTLP Log exporter: %w", err)
	}

	return traceExporter, metricExporter, logsExporter, nil
}

func configureLocalExporter() (sdktrace.SpanExporter, sdkmetric.Exporter, sdklog.Exporter, error) {
	traceFileDir := options.DefaultTmpDir()
	traceFileName := fmt.Sprintf("%s/trace-%s.jsonl", traceFileDir, time.Now().Format("20060102150405"))

	cfg, err := oteljsonl.NewConfig(
		oteljsonl.WithPath(traceFileName),
		oteljsonl.WithCreateDirs(true),
		oteljsonl.WithFileMode(0o600),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to configure trace output file %q: %w", traceFileName, err)
	}

	exporters, err := oteljsonl.NewExporters(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize trace exporter for %q: %w", traceFileName, err)
	}

	return exporters.Trace, exporters.Metric, exporters.Log, nil
}

func CreateLogsRemoteExporter(
	ctx context.Context,
	eType exporterType,
	exporterEndpointAuthorization string,
) (sdklog.Exporter, error) {
	var (
		exporter sdklog.Exporter
		err      error
	)

	switch eType {
	case grpcExporter:
		var opts []otlploggrpc.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlploggrpc.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlploggrpc.New(ctx, opts...)
	case httpExporter:
		fallthrough
	default:
		var opts []otlploghttp.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlploghttp.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlploghttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP Logs exporter: %w", err)
	}

	return exporter, nil
}

func CreateMetricRemoteExporter(
	ctx context.Context,
	eType exporterType,
	exporterEndpointAuthorization string,
) (sdkmetric.Exporter, error) {
	var (
		exporter sdkmetric.Exporter
		err      error
	)

	switch eType {
	case grpcExporter:
		var opts []otlpmetricgrpc.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlpmetricgrpc.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlpmetricgrpc.New(ctx, opts...)
	case httpExporter:
		fallthrough
	default:
		var opts []otlpmetrichttp.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlpmetrichttp.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlpmetrichttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP Metric exporter: %w", err)
	}

	return exporter, nil
}

func CreateTraceRemoteExporter(
	ctx context.Context,
	eType exporterType,
	exporterEndpointAuthorization string,
) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	switch eType {
	case grpcExporter:
		var opts []otlptracegrpc.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlptracegrpc.New(ctx, opts...)
	case httpExporter:
		fallthrough
	default:
		var opts []otlptracehttp.Option
		if exporterEndpointAuthorization != "" {
			opts = append(opts, otlptracehttp.WithHeaders(map[string]string{"Authorization": exporterEndpointAuthorization}))
		}

		exporter, err = otlptracehttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP Trace exporter: %w", err)
	}

	return exporter, nil
}
