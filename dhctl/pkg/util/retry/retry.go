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

package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/name212/govalue"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	attemptMessage = `Attempt #%d of %d |
	%s check attempt, retry in %v"
`
	NotSetName = "Name not set"
)

var InTestEnvironment = false

func setupTests(attemptsQuantity *int, wait *time.Duration) {
	if InTestEnvironment {
		*attemptsQuantity = 1
		*wait = 0 * time.Second
	}
}

type BreakPredicate func(err error) bool

func IsErr(err error) BreakPredicate {
	return func(target error) bool {
		return errors.Is(err, target)
	}
}

type Params interface {
	WithName(n string) Params
	WithAttempts(attempts int) Params
	WithWait(wait time.Duration) Params

	Name() string
	Attempts() int
	Wait() time.Duration

	Clone() Params
	Fill(c Params) Params
}

type params struct {
	name     string
	attempts int
	wait     time.Duration
}

type ParamsBuilderOpt func(Params)

func WithName(name string) ParamsBuilderOpt {
	return func(p Params) {
		p.WithName(name)
	}
}

func WithAttempts(attempts int) ParamsBuilderOpt {
	return func(p Params) {
		p.WithAttempts(attempts)
	}
}

func AttemptsWithWaitOpts(attempts int, wait time.Duration) []ParamsBuilderOpt {
	return []ParamsBuilderOpt{
		WithAttempts(attempts),
		WithWait(wait),
	}
}

func WithWait(wait time.Duration) ParamsBuilderOpt {
	return func(p Params) {
		p.WithWait(wait)
	}
}

func NewParams(name string, attempts int, wait time.Duration) Params {
	return NewEmptyParams().
		WithName(name).
		WithAttempts(attempts).
		WithWait(wait)
}

func NewEmptyParams(opts ...ParamsBuilderOpt) Params {
	p := &params{
		name:     NotSetName,
		attempts: 1,
		wait:     1 * time.Second,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *params) WithName(n string) Params {
	if n != "" {
		p.name = n
	}

	return p
}

func (p *params) WithAttempts(attempts int) Params {
	if attempts > 0 {
		p.attempts = attempts
	}

	return p
}

func (p *params) WithWait(wait time.Duration) Params {
	if wait > 0 {
		p.wait = wait
	}
	return p
}

func (p *params) Name() string {
	return p.name
}

func (p *params) Attempts() int {
	return p.attempts
}

func (p *params) Wait() time.Duration {
	return p.wait
}

func (p *params) Clone() Params {
	if govalue.IsNil(p) {
		return nil
	}

	return NewParams(p.Name(), p.Attempts(), p.Wait())
}

func (p *params) Fill(c Params) Params {
	if govalue.IsNil(p) {
		return nil
	}

	if govalue.IsNil(c) {
		return p
	}

	return p.WithName(c.Name()).
		WithAttempts(c.Attempts()).
		WithWait(c.Wait())
}

func SafeCloneOrNewParams(p Params, opts ...ParamsBuilderOpt) Params {
	if !govalue.IsNil(p) {
		return p.Clone()
	}

	return NewEmptyParams(opts...)
}

// Loop retries a task function until it succeeded with number of attempts and delay between runs are adjustable.
type Loop struct {
	name             string
	attemptsQuantity int
	waitTime         time.Duration
	breakPredicate   BreakPredicate
	interruptable    bool
	showError        bool
	prefix           string
	// silent keeps the loop off the compact terminal entirely: no framed process box and a
	// Debug-level (file-only) success line, so internal retries don't clutter the live logbox.
	silent bool
}

// NewLoop create Loop with features:
// - it is "verbose" loop — it prints messages through logboek.
// - this loop is interruptable by the signal watcher in tomb package.
func NewLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	return &Loop{
		name:             name,
		attemptsQuantity: attemptsQuantity,
		waitTime:         wait,
		interruptable:    true,
		showError:        true,
	}
}

func NewLoopWithParams(params Params) *Loop {
	p := params
	if govalue.IsNil(p) {
		p = NewEmptyParams()
	}

	return NewLoop(p.Name(), p.Attempts(), params.Wait())
}

func NewLoopWithParamsOpts(opts ...ParamsBuilderOpt) *Loop {
	return NewLoopWithParams(NewEmptyParams(opts...))
}

// NewSilentLoop create Loop with features:
// - it is "silent" loop — no messages are printed through logboek.
// - this loop is not interruptable by the signal watcher in tomb package.
func NewSilentLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	return &Loop{
		name:             name,
		attemptsQuantity: attemptsQuantity,
		waitTime:         wait,
		// - this loop is not interruptable by the signal watcher in tomb package.
		interruptable: false,
		showError:     true,
		prefix:        fmt.Sprintf("[%s][%d] ", name, rand.Int()),
		silent:        true,
	}
}

func NewSilentLoopWithParams(params Params) *Loop {
	p := params
	if govalue.IsNil(p) {
		p = NewEmptyParams()
	}

	return NewSilentLoop(p.Name(), p.Attempts(), p.Wait())
}

func NewSilentLoopWithParamsOpts(opts ...ParamsBuilderOpt) *Loop {
	return NewSilentLoopWithParams(NewEmptyParams(opts...))
}

func (l *Loop) BreakIf(pred BreakPredicate) *Loop {
	l.breakPredicate = pred
	return l
}

func (l *Loop) WithInterruptable(flag bool) *Loop {
	l.interruptable = flag
	return l
}

func (l *Loop) WithShowError(flag bool) *Loop {
	l.showError = flag
	return l
}

func (l *Loop) Run(task func() error) error {
	return l.run(context.Background(), task)
}

// RunContext retries a task like Run but breaks if context done.
func (l *Loop) RunContext(ctx context.Context, task func() error) error {
	return l.run(ctx, task)
}

func (l *Loop) run(ctx context.Context, task func() error) error {
	setupTests(&l.attemptsQuantity, &l.waitTime)

	if l.attemptsQuantity < 1 {
		return fmt.Errorf("Attempts quantity must be greater than zero for loop '%s'", l.name)
	}

	loopBody := func(ctx context.Context) error {
		var err error
		for i := 1; i <= l.attemptsQuantity; i++ {
			// Check if process is interrupted.
			if l.interruptable && tomb.IsInterrupted() {
				return fmt.Errorf("Loop was canceled: graceful shutdown")
			}

			// Run task and return if everything is ok.
			err = task()
			if err == nil {
				// A silent loop keeps its success file-only (Debug); a verbose loop surfaces it.
				if l.silent {
					dhlog.FromContext(ctx).DebugContext(ctx, l.prefix+"Succeeded!")
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, l.prefix+"Succeeded!")
				}
				return nil
			}

			if l.breakPredicate != nil && l.breakPredicate(err) {
				dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(l.prefix+"Client broke the loop with %v", err))
				return err
			}

			// Per-attempt diagnostics are logged at Debug: they enrich the debug file but never reach
			// the terminal — not even with -v (the terminal floor is Info, which -v does not lower).
			// The terminal stays quiet during retries; the final exhaustion error (returned below) is
			// what the caller surfaces if every attempt fails.
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(l.prefix+attemptMessage, i, l.attemptsQuantity, l.name, l.waitTime))
			errorMsg := "\t%v\n\n"
			if l.showError {
				errorMsg = "\tStatus: %v\n\n"
			}
			dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf(l.prefix+errorMsg, err), "\n"))

			// Do not waitTime after the last iteration.
			if i < l.attemptsQuantity {
				select {
				case <-time.After(l.waitTime):
				case <-ctx.Done():
					return fmt.Errorf("Loop was canceled: %w", ctx.Err())
				}
			}
		}

		return fmt.Errorf("Timeout while %q: last error: %w", l.name, err)
	}

	// A silent loop runs without a process block: no framed box and nothing on the compact terminal
	// (every per-attempt line is Debug → file-only). A verbose loop wraps the body in a process block
	// so its start/finish renders a framed box.
	if l.silent {
		return loopBody(ctx)
	}
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), l.name, loopBody)
}
