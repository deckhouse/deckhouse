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

package phases

import (
	"errors"
	"fmt"
	"sync"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

var (
	ErrShouldStop              = errors.New("should stop")
	ErrPipelineDidNotStart     = errors.New("pipeline did not start")
	ErrPipelineAlreadyFinished = errors.New("pipeline already finished")
	ErrPipelineAlreadyStarted  = errors.New("pipeline already started")
)

type (
	ActionFunc[OperationPhaseDataT any] func() (OperationPhaseDataT, error)

	PhaseAction[OperationPhaseDataT any] interface {
		// Run
		// if action should stop run should return ErrShouldStop
		Run(phase OperationPhase, isCritical bool, action ActionFunc[OperationPhaseDataT]) error
		CompleteSub(phase OperationSubPhase) error
	}

	ActionProvider[OperationPhaseDataT any] func() PhaseAction[OperationPhaseDataT]
)

type (
	DefaultActionFunc     = ActionFunc[DefaultContextType]
	DefaultPhaseAction    = PhaseAction[DefaultContextType]
	DefaultActionProvider = ActionProvider[DefaultContextType]
)

type PhaseActionWithStateCache[OperationPhaseDataT any] struct {
	stateCache   dstate.Cache
	phaseContext PhasedExecutionContext[OperationPhaseDataT]
}

func NewPhaseActionWithStateCache[OperationPhaseDataT any](context PhasedExecutionContext[OperationPhaseDataT], stateCache dstate.Cache) PhaseAction[OperationPhaseDataT] {
	return &PhaseActionWithStateCache[OperationPhaseDataT]{
		stateCache:   stateCache,
		phaseContext: context,
	}
}

func NewPhaseActionProviderWithStateCache[OperationPhaseDataT any](context PhasedExecutionContext[OperationPhaseDataT], stateCache dstate.Cache) ActionProvider[OperationPhaseDataT] {
	return func() PhaseAction[OperationPhaseDataT] {
		return NewPhaseActionWithStateCache[OperationPhaseDataT](context, stateCache)
	}
}

func NewDefaultPhaseActionProviderWithStateCache(context DefaultPhasedExecutionContext, stateCache dstate.Cache) DefaultActionProvider {
	return func() PhaseAction[DefaultContextType] {
		return NewPhaseActionWithStateCache(context, stateCache)
	}
}

func (a *PhaseActionWithStateCache[OperationPhaseDataT]) Run(phase OperationPhase, isCritical bool, action ActionFunc[OperationPhaseDataT]) error {
	if shouldStop, err := a.phaseContext.StartPhase(phase, isCritical, a.stateCache); err != nil {
		return err
	} else if shouldStop {
		return ErrShouldStop
	}

	completeData, err := action()

	if err != nil {
		return err
	}

	return a.phaseContext.CompletePhase(a.stateCache, completeData)
}

func (a *PhaseActionWithStateCache[OperationPhaseDataT]) CompleteSub(phase OperationSubPhase) error {
	a.phaseContext.CompleteSubPhase(phase)
	return nil
}

type dummyPhaseAction[OperationPhaseDataT any] struct{}

func (a *dummyPhaseAction[OperationPhaseDataT]) Run(_ OperationPhase, _ bool, action ActionFunc[OperationPhaseDataT]) error {
	_, err := action()
	return err
}

func (a *dummyPhaseAction[OperationPhaseDataT]) CompleteSub(OperationSubPhase) error {
	return nil
}

type phaseActionWithError[OperationPhaseDataT any] struct {
	err error
}

func newPhaseActionWithError[OperationPhaseDataT any](err error) PhaseAction[OperationPhaseDataT] {
	return &phaseActionWithError[OperationPhaseDataT]{
		err: err,
	}
}

func (a *phaseActionWithError[OperationPhaseDataT]) Run(phase OperationPhase, _ bool, _ ActionFunc[OperationPhaseDataT]) error {
	return fmt.Errorf("Phase '%s' cannot be run: %w", phase, a.err)
}

func (a *phaseActionWithError[OperationPhaseDataT]) CompleteSub(phase OperationSubPhase) error {
	return fmt.Errorf("SubPhase '%s' cannot be complete: %w", phase, a.err)
}

type (
	PipelinePhaseSwitcher[OperationPhaseDataT any] func(phase OperationPhase, isCritical bool, completedPhaseData OperationPhaseDataT) error

	PipelineAction[OperationPhaseDataT any] func(switcher PipelinePhaseSwitcher[OperationPhaseDataT]) error

	Pipeline[OperationPhaseDataT any] interface {
		// Run
		// should return ErrPipelineAlreadyFinished if call after run
		// should return ErrPipelineAlreadyStarted if call run inside run
		Run(action PipelineAction[OperationPhaseDataT]) error
		GetLastState() DhctlState
		// ActionInPipeline
		// can return with actions which returns ErrPipelineDidNotStart if call before call run
		ActionInPipeline() PhaseAction[OperationPhaseDataT]
		SetClusterConfig(cfg ClusterConfig)
	}

	PipelineProvider[OperationPhaseDataT any]         func(opts ...PipelineOptsFunc) Pipeline[OperationPhaseDataT]
	PreparedPipelineProvider[OperationPhaseDataT any] func() Pipeline[OperationPhaseDataT]
)

type (
	DefaultPipeline                 = Pipeline[DefaultContextType]
	DefaultPipelinePhaseSwitcher    = PipelinePhaseSwitcher[DefaultContextType]
	DefaultPipelineProvider         = PipelineProvider[DefaultContextType]
	PreparedDefaultPipelineProvider = PreparedPipelineProvider[DefaultContextType]
)

type PipelineOpts struct {
	LoggerProvider log.LoggerProvider
	PipelineName   string
}
type PipelineWithStateCache[OperationPhaseDataT any] struct {
	mu       sync.Mutex
	started  bool
	finished bool

	stateCache   dstate.Cache
	phaseContext PhasedExecutionContext[OperationPhaseDataT]
	opts         *PipelineOpts
}

type PipelineOptsFunc func(*PipelineOpts)

func WithPipelineLoggerProvider(provider log.LoggerProvider) PipelineOptsFunc {
	return func(opts *PipelineOpts) {
		opts.LoggerProvider = provider
	}
}

func WithPipelineName(name string) PipelineOptsFunc {
	return func(opts *PipelineOpts) {
		opts.PipelineName = name
	}
}

func NewPipelineWithStateCache[OperationPhaseDataT any](context PhasedExecutionContext[OperationPhaseDataT], stateCache dstate.Cache, opts ...PipelineOptsFunc) Pipeline[OperationPhaseDataT] {
	resultOpts := &PipelineOpts{
		PipelineName: "Pipeline name not set",
	}

	for _, opt := range opts {
		opt(resultOpts)
	}

	return &PipelineWithStateCache[OperationPhaseDataT]{
		stateCache:   stateCache,
		phaseContext: context,
		opts:         resultOpts,
	}
}

func newDummyPipeline[OperationPhaseDataT any](opts ...PipelineOptsFunc) Pipeline[OperationPhaseDataT] {
	return NewPipelineWithStateCache[OperationPhaseDataT](nil, nil, opts...)
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) Run(action PipelineAction[OperationPhaseDataT]) error {
	if p.isFinished() {
		return p.wrapError(ErrPipelineAlreadyFinished)
	}

	if p.isStarted() {
		return p.wrapError(ErrPipelineAlreadyStarted)
	}

	if err := p.initPipeline(); err != nil {
		return err
	}

	p.setStarted()
	defer p.setFinished()

	defer func() {
		p.finalize()
	}()

	err := action(p.phaseSwitcher)

	if govalue.IsNil(err) {
		return p.complete()
	}

	if errors.Is(err, ErrShouldStop) {
		log.SafeProvideLogger(p.opts.LoggerProvider).LogDebugF(
			"Pipeline '%s' with phase execution context: got should stop. Returns without complete\n",
			p.opts.PipelineName,
		)
		return nil
	}

	return err
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) GetLastState() DhctlState {
	if govalue.IsNil(p.phaseContext) {
		return DhctlState(nil)
	}

	return p.phaseContext.GetLastState()
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) SetClusterConfig(cfg ClusterConfig) {
	if govalue.IsNil(p.phaseContext) {
		return
	}

	p.phaseContext.SetClusterConfig(cfg)
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) ActionInPipeline() PhaseAction[OperationPhaseDataT] {
	if !p.isStarted() {
		return newPhaseActionWithError[OperationPhaseDataT](ErrPipelineDidNotStart)
	}

	if govalue.IsNil(p.phaseContext) {
		return &dummyPhaseAction[OperationPhaseDataT]{}
	}

	return NewPhaseActionWithStateCache(p.phaseContext, p.stateCache)
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) phaseSwitcher(phase OperationPhase, isCritical bool, completedPhaseData OperationPhaseDataT) error {
	if govalue.IsNil(p.phaseContext) {
		return nil
	}

	if shouldStop, err := p.phaseContext.SwitchPhase(phase, isCritical, p.stateCache, completedPhaseData); err != nil {
		return err
	} else if shouldStop {
		return ErrShouldStop
	}

	return nil
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) complete() error {
	if govalue.IsNil(p.phaseContext) {
		return nil
	}

	return p.phaseContext.CompletePipeline(p.stateCache)
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) finalize() {
	if govalue.IsNil(p.phaseContext) {
		return
	}

	if err := p.phaseContext.Finalize(p.stateCache); err != nil {
		log.SafeProvideLogger(p.opts.LoggerProvider).LogWarnF("Cannot finalize pipeline '%s': %v\n", p.opts.PipelineName, err)
	}
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) initPipeline() error {
	if govalue.IsNil(p.phaseContext) {
		return nil
	}

	if err := p.phaseContext.InitPipeline(p.stateCache); err != nil {
		return p.wrapError(fmt.Errorf("cannot init pipline: %w", err))
	}

	return nil
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) setStarted() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.started = true
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) isStarted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.started
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) setFinished() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.finished = true
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) isFinished() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.finished
}

func (p *PipelineWithStateCache[OperationPhaseDataT]) wrapError(err error) error {
	return fmt.Errorf("'%s': %w", p.opts.PipelineName, err)
}

func NewDefaultPipelineWithStateCache(context DefaultPhasedExecutionContext, stateCache dstate.Cache, opts ...PipelineOptsFunc) Pipeline[DefaultContextType] {
	return NewPipelineWithStateCache(context, stateCache, opts...)
}

func NewDefaultPipelineWithStateCacheProvider(context DefaultPhasedExecutionContext, stateCache dstate.Cache) DefaultPipelineProvider {
	return func(opts ...PipelineOptsFunc) Pipeline[DefaultContextType] {
		return NewPipelineWithStateCache(context, stateCache, opts...)
	}
}

func NewDefaultPipelineWithStateCacheProviderOpts(context DefaultPhasedExecutionContext, stateCache dstate.Cache, opts ...PipelineOptsFunc) PreparedDefaultPipelineProvider {
	return func() Pipeline[DefaultContextType] {
		return NewPipelineWithStateCache(context, stateCache, opts...)
	}
}

func NewDefaultPhaseActionProviderFromPipeline(pipeline Pipeline[DefaultContextType]) DefaultActionProvider {
	return func() PhaseAction[DefaultContextType] {
		return pipeline.ActionInPipeline()
	}
}

func NewPhaseActionProviderFromPipeline[OperationPhaseDataT any](pipeline Pipeline[OperationPhaseDataT]) ActionProvider[OperationPhaseDataT] {
	return func() PhaseAction[OperationPhaseDataT] {
		return pipeline.ActionInPipeline()
	}
}

func NewDummyDefaultPipelineProviderOpts(opts ...PipelineOptsFunc) PreparedDefaultPipelineProvider {
	return func() Pipeline[DefaultContextType] {
		return newDummyPipeline[DefaultContextType](opts...)
	}
}
