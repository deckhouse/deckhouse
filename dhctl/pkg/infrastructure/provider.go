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

package infrastructure

import (
	"context"
	"fmt"
	"sort"
	"sync"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

type AfterCleanupProviderFunc func()

type CloudProvider interface {
	NeedToUseTofu() bool
	OutputExecutor(ctx context.Context) (OutputExecutor, error)
	Executor(ctx context.Context, step Step) (Executor, error)
	// AddAfterCleanupFunc provider should sort groups before execution
	AddAfterCleanupFunc(group string, f AfterCleanupProviderFunc)
	Cleanup() error
	Name() string
	RootDir() string
	String() string
}

type DummyCloudProvider struct {
	cleanuper *AfterCleanupProviderRunner
}

func NewDummyCloudProvider() *DummyCloudProvider {
	return &DummyCloudProvider{
		cleanuper: NewAfterCleanupRunner("DummyCloudProvider"),
	}
}

func (p *DummyCloudProvider) Name() string {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Name on DummyCloudProvider")

	return "dummy"
}

func (p *DummyCloudProvider) NeedToUseTofu() bool {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call NeedToUseTofu on DummyCloudProvider")

	return false
}

func (p *DummyCloudProvider) OutputExecutor(ctx context.Context) (OutputExecutor, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call OutputExecutor on DummyCloudProvider")

	return NewDummyOutputExecutor(), nil
}

func (p *DummyCloudProvider) Executor(ctx context.Context, step Step) (Executor, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Executor on DummyCloudProvider")

	return NewDummyExecutor(), nil
}

func (p *DummyCloudProvider) Cleanup() error {
	p.cleanuper.Cleanup()
	return nil
}

func (p *DummyCloudProvider) RootDir() string {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call RootDir on DummyCloudProvider")

	return ""
}

func (p *DummyCloudProvider) String() string {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call String on DummyCloudProvider")

	return "dummy"
}

func (p *DummyCloudProvider) AddAfterCleanupFunc(group string, f AfterCleanupProviderFunc) {
	p.cleanuper.Add(group, f)
}

type AfterCleanupProviderRunner struct {
	providerName string

	afterCleanupMutex sync.Mutex
	afterCleanup      map[string][]AfterCleanupProviderFunc
}

func NewAfterCleanupRunner(providerName string) *AfterCleanupProviderRunner {
	return &AfterCleanupProviderRunner{
		providerName: providerName,
		afterCleanup: make(map[string][]AfterCleanupProviderFunc),
	}
}

func (r *AfterCleanupProviderRunner) Add(group string, f AfterCleanupProviderFunc) {
	r.afterCleanupMutex.Lock()
	defer r.afterCleanupMutex.Unlock()

	list, ok := r.afterCleanup[group]
	if !ok {
		list = make([]AfterCleanupProviderFunc, 0)
	}

	list = append(list, f)
	r.afterCleanup[group] = list
}

func (r *AfterCleanupProviderRunner) Cleanup() {
	r.afterCleanupMutex.Lock()
	defer r.afterCleanupMutex.Unlock()

	ctx := context.Background()

	groups := make([]string, 0, len(r.afterCleanup))
	for group := range r.afterCleanup {
		groups = append(groups, group)
	}

	sort.Strings(groups)

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Call AfterCleanupProviderRunner on %s. AfterCleanup functions groups %d %v in sorted order", r.providerName, len(r.afterCleanup), groups))
	for _, group := range groups {
		funcs := r.afterCleanup[group]
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Call cleanup functions %d on %s for group %s", len(funcs), r.providerName, group))
		for i, f := range funcs {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Call cleanup function %d on %s for group %s", i, r.providerName, group))
			f()
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Cleanup function %d on %s for group %s called", i, r.providerName, group))
		}

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Cleanup functions on %s for group %s called", r.providerName, group))
	}

	r.afterCleanup = make(map[string][]AfterCleanupProviderFunc)
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Call AfterCleanupProviderRunner on %s. AfterCleanup map was cleaned", r.providerName))
}
