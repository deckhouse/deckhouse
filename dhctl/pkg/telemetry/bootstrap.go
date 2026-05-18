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

	var (
		tracesExporter  sdktrace.SpanExporter
		metricsExporter sdkmetric.Exporter
		logsExporter    sdklog.Exporter
		err             error
	)

	if _, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); ok {
		tracesExporter, metricsExporter, logsExporter, err = configureRemoteExporter(ctx)
	} else {
		tracesExporter, metricsExporter, logsExporter, err = configureLocalExporter()
	}

	if err != nil {
		return err
	}

	otelResource, err := resource.New(
		ctx,
		resource.WithSchemaURL(semconv.SchemaURL),

		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),

		resource.WithHostID(),
		resource.WithOS(),
		resource.WithContainer(),

		resource.WithProcessPID(),
		resource.WithProcessOwner(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessCommandArgs(),

		resource.WithAttributes(
			semconv.ServiceName(traceApplicationName),
		),
	)
	if err != nil {
		// probably, we need to log this.
		// resource.New always returns *resource.Resource, but some data may be missed
	}

	tracesShutdown, _ := initTraces(tracesExporter, otelResource)
	metricsShutdown, _ := initMetrics(metricsExporter, otelResource)
	logsShutdown, _ := initLogs(logsExporter, otelResource)

	tomb.RegisterOnShutdown("OTel: traces", func() { tracesShutdown(ctx) })
	tomb.RegisterOnShutdown("OTel: metrics", func() { metricsShutdown(ctx) })
	tomb.RegisterOnShutdown("OTel: logs", func() { logsShutdown(ctx) })

	return nil
}

func initTraces(exporter sdktrace.SpanExporter, r *resource.Resource) (ShutdownFunc, error) {
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(provider)

	return func(ctx context.Context) error {
		if err := provider.ForceFlush(ctx); err != nil {
			return fmt.Errorf("failed to flush traces: %v", err)
		}
		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown traces: %v", err)
		}
		return nil
	}, nil
}

func initMetrics(exporter sdkmetric.Exporter, r *resource.Resource) (ShutdownFunc, error) {
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(r),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	otel.SetMeterProvider(provider)

	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		return nil, fmt.Errorf("failed to start runtime metrics: %v", err)
	}

	return func(ctx context.Context) error {
		if err := provider.ForceFlush(ctx); err != nil {
			return fmt.Errorf("failed to flush traces: %v", err)
		}
		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown traces: %v", err)
		}
		return nil
	}, nil
}

func initLogs(exporter sdklog.Exporter, r *resource.Resource) (ShutdownFunc, error) {
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

	return func(ctx context.Context) error {
		if err := provider.ForceFlush(ctx); err != nil {
			return fmt.Errorf("failed to flush traces: %v", err)
		}
		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown traces: %v", err)
		}
		return nil
	}, nil
}
