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

package registryscaner

import (
	"context"
	"registry-modules-watcher/internal/backends"
	"registry-modules-watcher/internal/backends/pkg/registry-scaner/cache"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type Client interface {
	Name() string
	ReleaseImage(moduleName, releaseChannel string) (v1.Image, error)
	Image(moduleName, version string) (v1.Image, error)
	ListTags(moduleName string) ([]string, error)
	Modules() ([]string, error)
}

type registryscaner struct {
	registryClients map[string]Client
	updateHandler   func([]backends.Version) error
	cache           *cache.Cache
}

var releaseChannelsTags = map[string]string{
	"alpha":        "",
	"beta":         "",
	"early-access": "",
	"rock-solid":   "",
	"stable":       "",
}

// New
func New(registryClients ...Client) *registryscaner {
	registryscaner := registryscaner{
		registryClients: make(map[string]Client),
		cache:           cache.New(),
	}

	for _, client := range registryClients {
		registryscaner.registryClients[client.Name()] = client
	}

	return &registryscaner
}

func (s *registryscaner) GetState() []backends.Version {
	return s.cache.GetState()
}

func (s *registryscaner) SubscribeOnUpdate(updateHandler func([]backends.Version) error) {
	s.updateHandler = updateHandler
}

// Subscribe
func (s *registryscaner) Subscribe(ctx context.Context, scanInterval time.Duration) {
	s.processRegistries(ctx)
	s.cache.ResetRange()
	ticker := time.NewTicker(scanInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				s.processRegistries(ctx)
				state := s.cache.GetRange()
				if len(state) > 0 {
					klog.V(3).Infof("new versions in registry found")
					s.updateHandler(state)
					s.cache.ResetRange()
				}

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
