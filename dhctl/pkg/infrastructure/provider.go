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

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type CloudProvider interface {
	NeedToUseTofu() bool
	OutputExecutor(ctx context.Context, logger log.Logger) (OutputExecutor, error)
	Executor(ctx context.Context, step Step, logger log.Logger) (Executor, error)
	Cleanup() error
	Name() string
	RootDir() string
	String() string
}

type DummyCloudProvider struct {
	logger log.Logger
}

func NewDummyCloudProvider(logger log.Logger) *DummyCloudProvider {
	return &DummyCloudProvider{
		logger: logger,
	}
}

func (p *DummyCloudProvider) Name() string {
	p.logger.LogWarnF("Call Name on DummyCloudProvider")

	return "dummy"
}
func (p *DummyCloudProvider) NeedToUseTofu() bool {
	p.logger.LogWarnF("Call NeedToUseTofu on DummyCloudProvider")

	return false
}
func (p *DummyCloudProvider) OutputExecutor(ctx context.Context, logger log.Logger) (OutputExecutor, error) {
	p.logger.LogWarnF("Call OutputExecutor on DummyCloudProvider")

	return NewDummyOutputExecutor(p.logger), nil
}
func (p *DummyCloudProvider) Executor(ctx context.Context, step Step, logger log.Logger) (Executor, error) {
	p.logger.LogWarnF("Call Executor on DummyCloudProvider")

	return NewDummyExecutor(logger), nil
}
func (p *DummyCloudProvider) Cleanup() error {
	return nil
}

func (p *DummyCloudProvider) RootDir() string {
	p.logger.LogWarnF("Call RootDir on DummyCloudProvider")

	return ""
}

func (p *DummyCloudProvider) String() string {
	p.logger.LogWarnF("Call String on DummyCloudProvider")

	return ""
}
