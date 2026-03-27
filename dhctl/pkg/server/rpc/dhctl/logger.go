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

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
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

func initDhctlLogger(ctx context.Context, action actionForInitLogger) log.Logger {
	logOptions := action.loggerOptions(ctx)

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   logOptions.DefaultWriter,
		Width:       logOptions.Width,
		DebugStream: logOptions.DebugWriter,
	})

	return log.GetDefaultLogger()
}
