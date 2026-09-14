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

package watchdog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)


const defaultIdentityCheckInterval = time.Second


var errIdentityChanged = errors.New("own node was recreated with a different uid")

type IdentityGuard struct {
	state    StateSource
	events   EventRecorder
	interval time.Duration
	logger   *log.Logger
}

func NewIdentityGuard(state StateSource, events EventRecorder, interval time.Duration, logger *log.Logger) *IdentityGuard {
	if interval <= 0 {
		interval = defaultIdentityCheckInterval
	}

	return &IdentityGuard{state: state, events: events, interval: interval, logger: logger}
}

func (g *IdentityGuard) Run(ctx context.Context) error {
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		if err := g.check(); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (g *IdentityGuard) check() error {
	snapshot := g.state.Snapshot()
	if !snapshot.UIDMismatch {
		return nil
	}

	g.logger.Error("own node was recreated with a different uid, restarting the agent")
	g.events.Warning(reasonIdentityChanged, "Node was recreated with a different uid, restarting the agent")

	return fmt.Errorf("%w: the identity and the SLA profile read at startup no longer describe this machine", errIdentityChanged)
}
