/*
Copyright 2026 Flant JSC

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

// Package agent is the registry-cache replication agent: a follower mirrors the
// leader's store (copy missing tags + prune stale ones) and every replica runs
// docker-distribution garbage collection on its local store.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync/atomic"
	"time"

	"sigs.k8s.io/yaml"

	"syncer/pkg/config"
	"syncer/pkg/syncer"
)

// User is a registry credential.
type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// Config is the cache-agent's mounted configuration.
type Config struct {
	LeaderAddress       string `json:"leaderAddress"`       // e.g. registry-cache-leader.d8-system.svc:5001
	LocalAddress        string `json:"localAddress"`        // e.g. 127.0.0.1:5001
	ReadUser            User   `json:"readUser"`            // ro: read from leader
	WriteUser           User   `json:"writeUser"`           // rw: write+delete locally
	CA                  string `json:"ca"`                  // module CA (PEM)
	SyncIntervalSeconds int    `json:"syncIntervalSeconds"` // default 60
	GCIntervalSeconds   int    `json:"gcIntervalSeconds"`   // default 3600
	RegistryBinary      string `json:"registryBinary"`      // default /registry
	DistributionConfig  string `json:"distributionConfig"`  // default /config/config.yaml
}

// LoadConfig reads the agent config YAML and applies defaults.
func LoadConfig(raw []byte) (Config, error) {
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("parse agent config: %w", err)
	}
	if c.SyncIntervalSeconds == 0 {
		c.SyncIntervalSeconds = 60
	}
	if c.GCIntervalSeconds == 0 {
		c.GCIntervalSeconds = 3600
	}
	if c.RegistryBinary == "" {
		c.RegistryBinary = "/registry"
	}
	if c.DistributionConfig == "" {
		c.DistributionConfig = "/config/config.yaml"
	}
	return c, nil
}

// Agent runs replication + GC. isLeader gates sync (the leader is the source and
// does not mirror from itself).
type Agent struct {
	cfg      Config
	logger   *slog.Logger
	isLeader *atomic.Bool
}

func New(logger *slog.Logger, cfg Config, isLeader *atomic.Bool) *Agent {
	return &Agent{cfg: cfg, logger: logger, isLeader: isLeader}
}

// syncConfig builds the syncer config: read from leader (ro), mirror+prune into
// the local store (rw).
func (a *Agent) syncConfig() config.Config {
	return config.Config{
		Src: config.Registry{
			Address: a.cfg.LeaderAddress,
			User:    &config.User{Name: a.cfg.ReadUser.Name, Password: a.cfg.ReadUser.Password},
			CA:      a.cfg.CA,
		},
		Dest: config.Registry{
			Address: a.cfg.LocalAddress,
			User:    &config.User{Name: a.cfg.WriteUser.Name, Password: a.cfg.WriteUser.Password},
			CA:      a.cfg.CA,
		},
		Prune: true,
	}
}

// RunSyncOnce mirrors the leader into the local store. A no-op for the leader.
func (a *Agent) RunSyncOnce(ctx context.Context) error {
	if a.isLeader.Load() {
		a.logger.Debug("leader: skipping mirror (self is source of truth)")
		return nil
	}
	s, err := syncer.New(a.logger, a.syncConfig())
	if err != nil {
		return fmt.Errorf("build syncer: %w", err)
	}
	return s.Run(ctx)
}

// RunGCOnce runs docker-distribution garbage collection on the local store.
func (a *Agent) RunGCOnce(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, a.cfg.RegistryBinary, "garbage-collect", "--delete-untagged", a.cfg.DistributionConfig)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("garbage-collect: %w: %s", err, string(out))
	}
	a.logger.Info("garbage-collect completed", "output", string(out))
	return nil
}

// RunLoops runs the sync and GC tickers until ctx is cancelled.
func (a *Agent) RunLoops(ctx context.Context) {
	syncTick := time.NewTicker(time.Duration(a.cfg.SyncIntervalSeconds) * time.Second)
	gcTick := time.NewTicker(time.Duration(a.cfg.GCIntervalSeconds) * time.Second)
	defer syncTick.Stop()
	defer gcTick.Stop()
	// Initial mirror so a fresh follower converges immediately rather than
	// waiting a full sync interval (matters during master rollout).
	if err := a.RunSyncOnce(ctx); err != nil {
		a.logger.Error("initial mirror failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-syncTick.C:
			if err := a.RunSyncOnce(ctx); err != nil {
				a.logger.Error("mirror failed", "error", err)
			}
		case <-gcTick.C:
			if err := a.RunGCOnce(ctx); err != nil {
				a.logger.Error("gc failed", "error", err)
			}
		}
	}
}
