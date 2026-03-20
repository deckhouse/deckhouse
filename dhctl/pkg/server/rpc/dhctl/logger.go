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

	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
)

type loggerProvider[T any] func(ctx context.Context) logger.Options

type logAttributesProvider interface {
	commanderClusterUUID() string
}

type loggerProviderParams[T any] struct {
	sendCh             chan T
	consumer           logger.LogConsumer[T]
	attributesProvider logAttributesProvider
}

func newLoggerProvider[T any](params *loggerProviderParams[T]) loggerProvider[T] {
	return func(ctx context.Context) logger.Options {
		logTypeDHCTL := slog.String("type", "dhctl")
		logCommanderID := slog.String("commander_cluster_uuid", params.attributesProvider.commanderClusterUUID())

		loggerDefault := logger.L(ctx).With(logTypeDHCTL, logCommanderID)

		logWriter := logger.NewLogWriter(loggerDefault, params.sendCh, params.consumer)

		debugWriter := logger.NewDebugLogWriter(loggerDefault)

		return logger.Options{
			DebugWriter:   debugWriter,
			DefaultWriter: logWriter,
		}
	}
}
