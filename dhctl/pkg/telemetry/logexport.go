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
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log/global"
)

// WithOTLPLogExport returns a logger whose records are mirrored to OTLP in
// addition to root's existing sinks (the file + TTY handler). Every record the
// root emits — all levels including DEBUG, plus klog and shell-operator once
// they are bound to it — is exported, so the OTLP stream is an exact mirror of
// the debug log file.
//
// It is a no-op when telemetry is disabled: root is returned unchanged.
//
// Must be called after Bootstrap has installed the global LoggerProvider. That
// ordering is guaranteed in the CLI: Bootstrap runs at process start, before the
// root logger is constructed during flag parsing.
func WithOTLPLogExport(root *slog.Logger) *slog.Logger {
	if !IsEnabled() {
		return root
	}

	// otelslog.Handle forwards the context to LoggerProvider.Emit, and the log
	// SDK reads the active span from it — so exported records are correlated to
	// the current dhctl span without any extra wiring here.
	otelHandler := otelslog.NewHandler(
		traceApplicationName,
		otelslog.WithLoggerProvider(global.GetLoggerProvider()),
		otelslog.WithSource(true),
	)

	// slog.MultiHandler (Go 1.26 stdlib) fans out: Enabled is OR over children,
	// so the file sink (DEBUG) keeps DEBUG records flowing to both sinks, and a
	// failure in one sink never drops the other.
	return slog.New(slog.NewMultiHandler(root.Handler(), otelHandler))
}
