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
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
)

const resourceExhaustedTimeout = time.Second

func PanicRecoveryHandler() func(ctx context.Context, p any) error {
	return func(ctx context.Context, p any) error {
		logger.L(ctx).Error(
			"recovered from panic",
			slog.Any("panic", p),
			slog.Any("stack", string(debug.Stack())),
		)
		return status.Errorf(codes.Internal, "%s", p)
	}
}

func UnaryLogger(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(logger.ToContext(ctx, log), req)
	}
}

func StreamLogger(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wss := newStreamContextWrapper(ss)
		wss.SetContext(logger.ToContext(ss.Context(), log))
		return handler(srv, wss)
	}
}

func Logger() logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		logger.L(ctx).Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func UnaryParallelTasksLimiter(sem chan struct{}) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if !strings.Contains(info.FullMethod, "dhctl") {
			return handler(ctx, req)
		}

		log := logger.L(ctx)
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

func StreamParallelTasksLimiter(sem chan struct{}) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !strings.Contains(info.FullMethod, "dhctl") {
			return handler(srv, ss)
		}

		log := logger.L(ss.Context())
		log.Info("limiter tries to start operation", slog.Int("concurrent_operation", len(sem)))
		timeout := time.After(resourceExhaustedTimeout)
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

type StreamContextWrapper interface {
	grpc.ServerStream
	SetContext(context.Context)
}

type wrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrapper) Context() context.Context {
	return w.ctx
}

func (w *wrapper) SetContext(ctx context.Context) {
	w.ctx = ctx
}

func newStreamContextWrapper(inner grpc.ServerStream) StreamContextWrapper {
	ctx := inner.Context()
	return &wrapper{
		ServerStream: inner,
		ctx:          ctx,
	}
}
