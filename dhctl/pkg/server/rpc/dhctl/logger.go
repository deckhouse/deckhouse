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

package dhctl

import (
	"context"
	"log/slog"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
)

type logAttributesProvider interface {
	loggerWidth() int
}

type actionForInitLogger interface {
	logAttributesProvider
	loggerOptions(ctx context.Context) logger.Options
}

type initLoggerOptionsParams[T any] struct {
	sendCh             chan T
	consumer           logger.LogConsumer[T]
	attributesProvider logAttributesProvider
}

// initLoggerOptions
// we could be to use actionForInitLogger to provide initLoggerOptionsParams
// and call initLoggerOptions inside initDhctlLogger func
// but all operations struct should be generic it is not good
// That's why we call initLoggerOptions inside every implementations of actionForInitLogger
func initLoggerOptions[T any](ctx context.Context, params *initLoggerOptionsParams[T]) logger.Options {
	logTypeDHCTL := slog.String("type", "dhctl")

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, params.sendCh, params.consumer)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	return logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
		Width:         params.attributesProvider.loggerWidth(),
	}
}

// initDhctlLoggerCtx returns a context carrying a streaming *slog.Logger so operations that
// log via dhlog.FromContext(ctx) reach the gRPC client stream.
//
// The streaming slog logger writes text records to opts.DefaultWriter (a LogWriter): it splits
// the records into lines, logs each to the server slog, and streams them to the client over
// sendCh. No slog.SetDefault is needed — handlers and operations read the logger from ctx.
func initDhctlLoggerCtx(ctx context.Context, action actionForInitLogger) context.Context {
	opts := action.loggerOptions(ctx)

	// streaming slog logger over the client-stream writer, carried on ctx for operations.
	// NewStreamLogger renders the compact logboek UI (process boxes, milestones) so the commander
	// client receives the pretty tree format instead of raw slog text.
	streamLogger := dhlog.NewStreamLogger(opts.DefaultWriter)
	ctx = dhlog.ToContext(ctx, streamLogger)

	return ctx
}
