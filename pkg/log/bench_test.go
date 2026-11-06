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

package log_test

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func newBenchLogger(level slog.Level, handlerType log.HandlerType) *log.Logger {
	return log.NewLogger(
		log.WithLevel(level),
		log.WithOutput(io.Discard),
		log.WithHandlerType(handlerType),
	)
}

// a typical line: a named logger with a couple of attributes and four call attributes
func benchmarkInfo(b *testing.B, handlerType log.HandlerType) {
	l := newBenchLogger(slog.LevelInfo, handlerType).
		Named("addon-operator").Named("module-manager").
		With(slog.String("module", "deckhouse"), slog.String("hook", "040-node-manager/hooks/foo.go"))
	err := errors.New("boom")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Info("module run finished",
			slog.String("phase", "converge"),
			slog.Int("queue_len", 42),
			slog.Duration("took", 1500*time.Millisecond),
			log.Err(err),
		)
	}
}

func BenchmarkInfoJSON(b *testing.B) { benchmarkInfo(b, log.JSONHandlerType) }

func BenchmarkInfoText(b *testing.B) { benchmarkInfo(b, log.TextHandlerType) }

// shell-operator hook loggers carry many attributes
func BenchmarkInfoJSONManyAttrs(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType).
		With("hook", "040-node-manager/hooks/foo.go", "binding", "kubernetes", "event.type", "Event",
			"hook.type", "module", "module", "node-manager", "queue", "main",
			"task.id", "6d685def-ad8e-4961-a14b-a422e1f76a31", "output", "stdout")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Info("hook result", slog.String("phase", "converge"))
	}
}

func BenchmarkInfoJSONParallel(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType).Named("bench")

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info("module run finished", slog.String("phase", "converge"), slog.Int("queue_len", 42))
		}
	})
}

// sources are written at debug level
func BenchmarkDebugEnabledJSON(b *testing.B) {
	l := newBenchLogger(slog.LevelDebug, log.JSONHandlerType).Named("bench")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Debug("debug line", slog.String("phase", "converge"), slog.Int("queue_len", 42))
	}
}

func BenchmarkDebugDisabled(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType).Named("bench")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Debug("debug line", slog.String("phase", "converge"), slog.Int("queue_len", 42))
	}
}

func BenchmarkGlobalInfo(b *testing.B) {
	prev := log.Default()
	log.SetDefault(newBenchLogger(slog.LevelInfo, log.JSONHandlerType))
	b.Cleanup(func() { log.SetDefault(prev) })

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		log.Info("global line", slog.String("phase", "converge"), slog.Int("queue_len", 42))
	}
}

func BenchmarkGlobalDebugDisabled(b *testing.B) {
	prev := log.Default()
	log.SetDefault(newBenchLogger(slog.LevelInfo, log.JSONHandlerType))
	b.Cleanup(func() { log.SetDefault(prev) })

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		log.Debug("global line", slog.String("phase", "converge"), slog.Int("queue_len", 42))
	}
}

func BenchmarkErrorJSON(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType).Named("bench")
	err := errors.New("boom")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Error("reconcile failed", log.Err(err), slog.String("module", "deckhouse"))
	}
}

var benchValuesJSON = `{"global":{"clusterConfiguration":{"apiVersion":"deckhouse.io/v1","kind":"ClusterConfiguration",` +
	`"clusterType":"Static","podSubnetCIDR":"10.111.0.0/16","serviceSubnetCIDR":"10.222.0.0/16","kubernetesVersion":"Automatic",` +
	`"clusterDomain":"cluster.local"},"modules":{"publicDomainTemplate":"%s.example.com","https":{"mode":"CertManager",` +
	`"certManager":{"clusterIssuerName":"letsencrypt"}}},"enabledModules":["` +
	strings.Repeat(`module-a","module-b","module-c","`, 30) + `last"]}}`

func BenchmarkRawJSONDebugDisabled(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Debug("values", log.RawJSON("values", benchValuesJSON))
	}
}

func BenchmarkRawJSONInfo(b *testing.B) {
	l := newBenchLogger(slog.LevelInfo, log.JSONHandlerType)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		l.Info("values", log.RawJSON("values", benchValuesJSON))
	}
}
