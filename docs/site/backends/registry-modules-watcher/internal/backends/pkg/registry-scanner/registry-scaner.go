// Copyright 2023 Flant JSC
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

package registryscanner

import (
	"context"
	"log/slog"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"registry-modules-watcher/internal/backends"
	"registry-modules-watcher/internal/backends/pkg/registry-scanner/cache"
)

type Client interface {
	Name() string
	ReleaseImage(ctx context.Context, moduleName, releaseChannel string) (v1.Image, error)
	Image(ctx context.Context, moduleName, version string) (v1.Image, error)
	ListTags(ctx context.Context, moduleName string) ([]string, error)
	Modules(ctx context.Context) ([]string, error)
}

type registryscanner struct {
	registryClients map[string]Client
	updateHandler   func([]backends.DocumentationTask) error
	cache           *cache.Cache

	logger *log.Logger
	ms     *metricsstorage.MetricStorage
}

var releaseChannelsTags = map[string]string{
	"alpha":        "",
	"beta":         "",
	"early-access": "",
	"rock-solid":   "",
	"stable":       "",
}

// New
// nolint: revive
func New(logger *log.Logger, ms *metricsstorage.MetricStorage, registryClients ...Client) *registryscanner {
	registryscanner := registryscanner{
		registryClients: make(map[string]Client),
		cache:           cache.New(ms),
		logger:          logger,
		ms:              ms,
	}

	for _, client := range registryClients {
		registryscanner.registryClients[client.Name()] = client
	}

	return &registryscanner
}

func (s *registryscanner) GetState() []backends.DocumentationTask {
	return s.cache.GetState()
}

func (s *registryscanner) SubscribeOnUpdate(updateHandler func([]backends.DocumentationTask) error) {
	s.updateHandler = updateHandler
}

func (s *registryscanner) Subscribe(ctx context.Context, scanInterval time.Duration) {
	// synchronous processing - wait for ready
	s.processRegistries(ctx)
	ticker := time.NewTicker(scanInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				docTask := s.processRegistries(ctx)
				if len(docTask) == 0 {
					continue
				}

				createCounter := 0
				deleteCounter := 0
				for _, task := range docTask {
					switch task.Task {
					case backends.TaskCreate:
						createCounter++
						s.logger.Info("received a new module version, processing...", slog.String("module", task.Module), slog.String("version", task.Version))
					case backends.TaskDelete:
						deleteCounter++
						s.logger.Info("find module version to remove, processing...", slog.String("module", task.Module), slog.String("version", task.Version))
					}
				}

				s.logger.Info("module versions changed in registry", slog.Int("create", createCounter), slog.Int("delete", deleteCounter))

				if err := s.updateHandler(docTask); err != nil {
					s.logger.Error("updateHandler", log.Err(err))
				}

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
