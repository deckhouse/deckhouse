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
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		log.Info("limiter tries to start task", slog.Int("concurrent_tasks", len(sem)))
		timeout := time.After(5 * time.Minute)
		select {
		case <-timeout:
			log.Info("limiter couldn't start task", slog.Int("concurrent_tasks", len(sem)))
			return nil, status.Error(codes.ResourceExhausted, "too many dhctl operation has already started")
		case sem <- struct{}{}:
			log.Info("limiter started task", slog.Int("concurrent_tasks", len(sem)))
			defer func() {
				<-sem
				log.Info("limiter finished task", slog.Int("concurrent_tasks", len(sem)))
			}()

			return handler(ctx, req)
		}
	}
}

func StreamParallelTasksLimiter(sem chan struct{}, log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		log.Info("limiter tries to start task", slog.Int("concurrent_tasks", len(sem)))
		timeout := time.After(5 * time.Minute)
		select {
		case <-timeout:
			log.Info("limiter couldn't start task", slog.Int("concurrent_tasks", len(sem)))
			return status.Error(codes.ResourceExhausted, "too many dhctl operation has already started")
		case sem <- struct{}{}:
			log.Info("limiter started task", slog.Int("concurrent_tasks", len(sem)))
			defer func() {
				<-sem
				log.Info("limiter finished task", slog.Int("concurrent_tasks", len(sem)))
			}()

			return handler(srv, ss)
		}
	}
}

func UnaryServerSinglefligt(m *sync.Mutex, log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !strings.Contains(info.FullMethod, "dhctl") {
			return handler(ctx, req)
		}

		log.Info("lock acquired", slog.String("method", info.FullMethod))
		locked := m.TryLock()
		if !locked {
			log.Info("couldn't acquire lock", slog.String("method", info.FullMethod))
			return nil, status.Error(codes.ResourceExhausted, "one dhctl operation has already started")
		}
		defer func() {
			m.Unlock()
			log.Info("lock released", slog.String("method", info.FullMethod))
		}()
		return handler(ctx, req)
	}
}

func StreamServerSinglefligt(m *sync.Mutex, log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !strings.Contains(info.FullMethod, "dhctl") {
			return handler(srv, ss)
		}

		log.Info("lock acquired", slog.String("method", info.FullMethod))
		locked := m.TryLock()
		if !locked {
			log.Info("couldn't acquire lock", slog.String("method", info.FullMethod))
			return status.Error(codes.ResourceExhausted, "one dhctl operation has already started")
		}
		defer func() {
			m.Unlock()
			log.Info("lock released", slog.String("method", info.FullMethod))
		}()
		return handler(srv, ss)
	}
}
