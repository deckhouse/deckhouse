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
				if len(state) == 0 {
					continue
				}
				s.updateHandler(state)
				s.cache.ResetRange()

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
