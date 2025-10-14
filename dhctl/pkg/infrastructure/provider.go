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
	"sort"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type AfterCleanupProviderFunc func(logger log.Logger)

type CloudProvider interface {
	NeedToUseTofu() bool
	OutputExecutor(ctx context.Context, logger log.Logger) (OutputExecutor, error)
	Executor(ctx context.Context, step Step, logger log.Logger) (Executor, error)
	// AddAfterCleanupFunc provider should sort groups before execution
	AddAfterCleanupFunc(group string, f AfterCleanupProviderFunc)
	Cleanup() error
	Name() string
	RootDir() string
	String() string
}

type DummyCloudProvider struct {
	logger log.Logger

	cleanuper *AfterCleanupProviderRunner
}

func NewDummyCloudProvider(logger log.Logger) *DummyCloudProvider {
	return &DummyCloudProvider{
		logger:    logger,
		cleanuper: NewAfterCleanupRunner("DummyCloudProvider"),
	}
}

func (p *DummyCloudProvider) Name() string {
	p.logger.LogWarnLn("Call Name on DummyCloudProvider")

	return "dummy"
}
func (p *DummyCloudProvider) NeedToUseTofu() bool {
	p.logger.LogWarnLn("Call NeedToUseTofu on DummyCloudProvider")

	return false
}
func (p *DummyCloudProvider) OutputExecutor(ctx context.Context, logger log.Logger) (OutputExecutor, error) {
	p.logger.LogWarnLn("Call OutputExecutor on DummyCloudProvider")

	return NewDummyOutputExecutor(p.logger), nil
}
func (p *DummyCloudProvider) Executor(ctx context.Context, step Step, logger log.Logger) (Executor, error) {
	p.logger.LogWarnLn("Call Executor on DummyCloudProvider")

	return NewDummyExecutor(logger), nil
}
func (p *DummyCloudProvider) Cleanup() error {
	p.cleanuper.Cleanup(p.logger)
	return nil
}

func (p *DummyCloudProvider) RootDir() string {
	p.logger.LogWarnLn("Call RootDir on DummyCloudProvider")

	return ""
}

func (p *DummyCloudProvider) String() string {
	p.logger.LogWarnLn("Call String on DummyCloudProvider")

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

func (r *AfterCleanupProviderRunner) Cleanup(logger log.Logger) {
	r.afterCleanupMutex.Lock()
	defer r.afterCleanupMutex.Unlock()

	groups := make([]string, 0, len(r.afterCleanup))
	for group := range r.afterCleanup {
		groups = append(groups, group)
	}

	sort.Strings(groups)

	logger.LogDebugF("Call AfterCleanupProviderRunner on %s. AfterCleanup functions groups %d %v in sorted order\n", r.providerName, len(r.afterCleanup), groups)
	for _, group := range groups {
		funcs := r.afterCleanup[group]
		logger.LogDebugF("Call cleanup functions %d on %s for group %s\n", len(funcs), r.providerName, group)
		for i, f := range funcs {
			logger.LogDebugF("Call cleanup function %d on %s for group %s\n", i, r.providerName, group)
			f(logger)
			logger.LogDebugF("Cleanup function %d on %s for group %s called\n", i, r.providerName, group)
		}

		logger.LogDebugF("Cleanup functions on %s for group %s called\n", r.providerName, group)
	}

	r.afterCleanup = make(map[string][]AfterCleanupProviderFunc)
	logger.LogDebugF("Call AfterCleanupProviderRunner on %s. AfterCleanup map was cleaned\n", r.providerName)
}
