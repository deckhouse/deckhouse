// Copyright 2025 Flant JSC
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

package operator

import (
	"context"
)

const (
	Pending Phase = "Pending" // wait for installing and loading
	Loaded  Phase = "Loaded"  // wait for startup
	Running Phase = "Running"
)

type Phase string

type Package struct {
	name string

	ctx    context.Context
	cancel context.CancelFunc

	status Status
}

type Status struct {
	Phase Phase `json:"phase" yaml:"phase"`
}

func (p *Package) renewContext(ctx context.Context) context.Context {
	if p.cancel != nil {
		p.cancel()
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	return p.ctx
}
