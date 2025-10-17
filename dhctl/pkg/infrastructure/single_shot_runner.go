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

package infrastructure

import (
	"context"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func NewSingleShotRunner(runner *Runner) *SingleShotRunner {
	return &SingleShotRunner{
		Runner: runner,
	}
}

type SingleShotRunner struct {
	*Runner

	init, apply, plan, destroy, stop sync.Once
}

func (r *SingleShotRunner) Init(ctx context.Context) (err error) {
	r.init.Do(func() {
		err = r.Runner.Init(ctx)
	})
	return
}

func (r *SingleShotRunner) Apply(ctx context.Context) (err error) {
	r.apply.Do(func() {
		err = r.Runner.Apply(ctx)
	})
	return
}

func (r *SingleShotRunner) Plan(ctx context.Context, destroy, noout bool) (err error) {
	r.plan.Do(func() {
		err = r.Runner.Plan(ctx, destroy, noout)
	})
	return
}

func (r *SingleShotRunner) GetInfrastructureOutput(ctx context.Context, output string) ([]byte, error) {
	return r.Runner.GetInfrastructureOutput(ctx, output)
}

func (r *SingleShotRunner) Destroy(ctx context.Context) (err error) {
	r.destroy.Do(func() {
		err = r.Runner.Destroy(ctx)
	})
	return
}

func (r *SingleShotRunner) ResourcesQuantityInState() int {
	return r.Runner.ResourcesQuantityInState()
}

func (r *SingleShotRunner) Stop() {
	r.stop.Do(func() {
		r.Runner.Stop()
	})
}

func (r *SingleShotRunner) GetLogger() log.Logger {
	return r.Runner.logger
}
