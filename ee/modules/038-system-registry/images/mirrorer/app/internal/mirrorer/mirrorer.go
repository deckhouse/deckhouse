/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mirrorer

import (
	"context"
	"fmt"
	"log/slog"
	"mirrorer/internal/config"
	"mirrorer/internal/syncer"
	"mirrorer/internal/transport"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Mirrorer = *mirrorer

type mirrorer struct {
	sleepInterval time.Duration
	syncers       []*syncer.Syncer
	log           *slog.Logger
}

func New(logger *slog.Logger, cfg config.Config) (Mirrorer, error) {
	ret := &mirrorer{}

	var caFiles []string
	if cfg.CAFile != "" {
		caFiles = append(caFiles, cfg.CAFile)
	}

	if cfg.SleepInterval > 0 {
		ret.sleepInterval = time.Duration(cfg.SleepInterval) * time.Second
	} else {
		ret.sleepInterval = 10 * time.Second
	}

	roundTripper, err := transport.NewHTTPRoundTripper(false, caFiles...)
	if err != nil {
		return nil, fmt.Errorf("cannot create http transport: %w", err)
	}

	localRegistry, err := name.NewRegistry(cfg.LocalAddress)
	if err != nil {
		return nil, fmt.Errorf("parse local registry address \"%v\" error: %w", cfg.LocalAddress, err)
	}

	localOptions := []remote.Option{
		remote.WithTransport(roundTripper),
		remote.WithAuth(&authn.Basic{
			Username: cfg.Users.Pusher.Name,
			Password: cfg.Users.Pusher.Password,
		}),
	}

	remoteOptions := []remote.Option{
		remote.WithTransport(roundTripper),
		remote.WithAuth(&authn.Basic{
			Username: cfg.Users.Puller.Name,
			Password: cfg.Users.Puller.Password,
		}),
	}

	for _, remoteRegistry := range cfg.RemoteAddresses {
		remoteRegistry, err := name.NewRegistry(remoteRegistry)
		if err != nil {
			return nil, fmt.Errorf("parse remote registry address \"%v\" error: %w", remoteRegistry, err)
		}

		ret.syncers = append(ret.syncers, &syncer.Syncer{
			Log:         logger.With("component", "syncer"),
			Src:         remoteRegistry,
			Dst:         localRegistry,
			SrcOptions:  remoteOptions,
			DstOptions:  localOptions,
			Parallelizm: cfg.Parallelizm,
		})
	}

	ret.log = logger.With("component", "mirrorer")

	return ret, nil
}

func (m *mirrorer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(m.sleepInterval):
			if err := m.doSync(ctx); err != nil {
				return err
			}
		}
	}
}

func (m *mirrorer) doSync(ctx context.Context) error {
	startTime := time.Now()
	m.log.Info("Mirror start")

	for _, sync := range m.syncers {
		if err := sync.Sync(ctx); err != nil {
			m.log.Error("Sync error", "error", err)
		}
	}
	m.log.Info("Mirror done", "duration", time.Since(startTime))

	return nil
}
