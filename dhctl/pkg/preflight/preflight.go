// Copyright 2026 Flant JSC
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

package preflightnew

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Preflight struct {
	suites    []Suite
	disabled  map[CheckName]struct{}
	cache     cache
	cacheSalt string
}

type cache interface {
	Save(string, []byte) error
	InCache(string) (bool, error)
}

func New(suites ...Suite) *Preflight {
	return &Preflight{
		suites:   append([]Suite(nil), suites...),
		disabled: make(map[CheckName]struct{}),
	}
}

func (p *Preflight) UseCache(cache cache) {
	p.cache = cache
}

func (p *Preflight) SetCacheSalt(salt string) {
	p.cacheSalt = salt
}

func (p *Preflight) AddSuite(suite Suite) {
	if suite == nil {
		return
	}
	p.suites = append(p.suites, suite)
}

func (p *Preflight) DisableCheck(name string) {
	p.disabled[CheckName(name)] = struct{}{}
}

func (p *Preflight) DisableChecks(names ...string) {
	for _, name := range names {
		p.DisableCheck(name)
	}
}

func (p *Preflight) IsDisabled(name string) bool {
	return p.isDisabled(CheckName(name))
}

func (p *Preflight) Run(ctx context.Context, phase Phase) error {
	checks, err := p.prepareChecks(phase)
	if err != nil {
		return err
	}
	phaseLabel := fmt.Sprintf("(%s)", phase)
	runFunc := func() error {
		return p.runChecks(ctx, checks)
	}
	return log.Process("preflight", phaseLabel, runFunc)
}

func (p *Preflight) runChecks(ctx context.Context, checks []Check) error {
	for _, check := range checks {
		if check.Disabled {
			log.InfoF("✓ %s: %s (skipped)\n", check.Name, check.Description)
			continue
		}
		if err := p.runCheck(ctx, check); err != nil {
			return err
		}
	}
	return nil
}

func (p *Preflight) runCheck(ctx context.Context, check Check) error {
	if p.cache != nil {
		key := p.cacheKey(check.Name)
		if ok, err := p.cache.InCache(key); err == nil && ok {
			log.InfoF("✓ %s: %s (cached)\n", check.Name, check.Description)
			return nil
		}
	}

	if err := p.retry(ctx, check); err != nil {
		return fmt.Errorf("preflight check %q failed.\nreason: %w", check.Name, err)
	}
	log.InfoF("✓ %s: %s\n", check.Name, check.Description)

	if p.cache != nil {
		if err := p.cache.Save(p.cacheKey(check.Name), []byte("yes")); err != nil {
			log.WarnF("cannot cache result of %s: %v\n", check.Name, err)
		}
	}
	return nil
}

func (p *Preflight) prepareChecks(phase Phase) ([]Check, error) {
	var checks []Check
	for _, suite := range p.suites {
		if suite == nil {
			continue
		}
		for _, check := range suite.Checks() {
			if err := check.Name.Validate(); err != nil {
				return nil, err
			}
			if check.Phase != phase {
				continue
			}
			if p.isDisabled(check.Name) {
				check.Disable()
			}
			checks = append(checks, check)
		}
	}
	return checks, nil
}

func (p *Preflight) isDisabled(name CheckName) bool {
	_, ok := p.disabled[name]
	return ok
}

func (p *Preflight) retry(ctx context.Context, check Check) error {
	attempts := check.Retry.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	var bo backoff.BackOff = backoff.NewExponentialBackOff(check.Retry.Options...)
	bo = backoff.WithMaxRetries(bo, uint64(attempts-1))
	bo = backoff.WithContext(bo, ctx)
	attempt := 0
	printedHeader := false
	return backoff.RetryNotify(
		func() error { attempt++; return check.Run(ctx) },
		bo,
		func(err error, next time.Duration) {
			if !printedHeader {
				log.WarnF("%s: %s\n\n", check.Name, check.Description)
				printedHeader = true
			}
			log.InfoF("retry %d/%d in %s\nreason: %v\n", attempt, attempts, next, err)
		},
	)
}

func (p *Preflight) cacheKey(name CheckName) string {
	if p.cacheSalt == "" {
		return fmt.Sprintf("preflight-%s", name)
	}
	return fmt.Sprintf("preflight-%s-%s", p.cacheSalt, name)
}
