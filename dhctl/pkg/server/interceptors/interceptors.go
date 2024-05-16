// Copyright 2024 Flant JSC
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

package interceptors

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	dhctllog "github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func PanicRecoveryHandler(log *slog.Logger) func(p any) error {
	return func(p any) error {
		log.Error(
			"recovered from panic",
			slog.Any("panic", p),
			slog.Any("stack", string(debug.Stack())),
		)
		return status.Errorf(codes.Internal, "%s", p)
	}
}

func Logger(log *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		log.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func UnaryParallelTasksLimiter(sem chan struct{}, log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		log.Info("limiter tries to start operation", slog.Int("concurrent_operation", len(sem)))
		timeout := time.After(5 * time.Minute)
		select {
		case <-timeout:
			log.Info("limiter couldn't start operation due to timeout", slog.Int("concurrent_operation", len(sem)))
			return nil, status.Error(codes.ResourceExhausted, "too many dhctl operation has already started")
		case sem <- struct{}{}:
			log.Info("limiter started operation", slog.Int("concurrent_operation", len(sem)))
			defer func() {
				<-sem
				log.Info("limiter finished operation", slog.Int("concurrent_operation", len(sem)))
			}()

			return handler(ctx, req)
		}
	}
}

func StreamParallelTasksLimiter(sem chan struct{}, log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		log.Info("limiter tries to start operation", slog.Int("concurrent_operation", len(sem)))
		timeout := time.After(5 * time.Minute)
		select {
		case <-timeout:
			log.Info("limiter couldn't start operation due to timeout", slog.Int("concurrent_operation", len(sem)))
			return status.Error(codes.ResourceExhausted, "too many dhctl operations has already started")
		case sem <- struct{}{}:
			log.Info("limiter started operation", slog.Int("concurrent_operation", len(sem)))
			defer func() {
				<-sem
				log.Info("limiter finished operation", slog.Int("concurrent_operation", len(sem)))
			}()

			return handler(srv, ss)
		}
	}
}
