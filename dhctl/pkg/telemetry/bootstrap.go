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
	"time"

	"github.com/090809/oteljsonl"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type ShutdownFunc func(ctx context.Context) error

func Bootstrap(ctx context.Context) error {
	traceValue, ok := os.LookupEnv("DHCTL_TRACE")
	if !ok || traceValue == "" || traceValue == "0" || traceValue == "no" {
		return nil
	}

	traceFileDir, ok := os.LookupEnv("DHCTL_TRACE_DIR")
	if !ok || traceFileDir != "" {
		traceFileDir, _ = os.Getwd()
	}

	traceFileName := fmt.Sprintf("%s/trace-%s.jsonl", traceFileDir, time.Now().Format("20060102150405"))

	cfg, err := oteljsonl.NewConfig(
		oteljsonl.WithPath(traceFileName),
		oteljsonl.WithCreateDirs(true),
		oteljsonl.WithFileMode(0o600),
	)
	if err != nil {
		return fmt.Errorf("failed to configure trace output file %q: %w", traceFileName, err)
	}

	exporters, err := oteljsonl.NewExporters(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize trace exporter for %q: %w", traceFileName, err)
	}

	tracesShutdown, _ := initTraces(exporters.Trace)
	metricsShutdown, _ := initMetrics(exporters.Metric)
	logsShutdown, _ := initLogs(exporters.Log)

	tomb.RegisterOnShutdown("OTel: traces", func() { tracesShutdown(ctx) })
	tomb.RegisterOnShutdown("OTel: metrics", func() { metricsShutdown(ctx) })
	tomb.RegisterOnShutdown("OTel: logs", func() { logsShutdown(ctx) })

	return nil
}

func initTraces(exporter sdktrace.SpanExporter) (ShutdownFunc, error) {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(traceApplicationName),
		),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(provider)

	return provider.Shutdown, nil
}

func initMetrics(exporter sdkmetric.Exporter) (ShutdownFunc, error) {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(traceApplicationName),
		),
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(r),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	otel.SetMeterProvider(provider)

	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		return nil, fmt.Errorf("failed to start runtime metrics: %v", err)
	}

	return provider.Shutdown, nil
}

func initLogs(exporter sdklog.Exporter) (ShutdownFunc, error) {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(traceApplicationName),
		),
	)

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(r),
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(exporter),
		),
	)

	global.SetLoggerProvider(provider)

	// todo: add to external logger?
	//slog.SetDefault(
	//	slog.New(
	//		slogmulti.Fanout(
	//			slog.Default().Handler(),
	//			otelslog.NewHandler(
	//				os.Getenv("APP_NAME"),
	//				otelslog.WithLoggerProvider(provider),
	//				otelslog.WithSource(true),
	//				otelslog.WithVersion(os.Getenv("CI_APPLICATION_TAG")),
	//			),
	//		),
	//	),
	//)

	return provider.Shutdown, nil
}
