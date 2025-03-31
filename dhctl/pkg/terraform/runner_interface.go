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

package terraform

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PlanOptions struct {
	Destroy bool
}

type RunnerInterface interface {
	Init(ctx context.Context) error
	Apply(ctx context.Context) error
	Plan(ctx context.Context, opts PlanOptions) error
	Destroy(ctx context.Context) error
	Stop()

	ResourcesQuantityInState() int
	GetTerraformOutput(ctx context.Context, output string) ([]byte, error)
	GetState() ([]byte, error)
	GetStep() string
	GetChangesInPlan() int
	GetPlanDestructiveChanges() *PlanDestructiveChanges
	GetPlanPath() string
	GetTerraformExecutor() Executor
	GetLogger() log.Logger
}
