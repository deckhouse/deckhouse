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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestPipelineWrapper(t *testing.T) {
	loggerProvider := log.DefaultLoggerProvider
	loggerProviderOpt := WithPipelineLoggerProvider(loggerProvider)

	pipelineNameOpt := WithPipelineName

	type (
		contextType  = DefaultContextType
		switcherType = PipelinePhaseSwitcher[contextType]
		actionType   = PipelineAction[contextType]
		pipelineType = Pipeline[contextType]

		beforeAfterTestType         = func() error
		afterTestActionProviderType = func(testName string, state dstate.Cache, pipeline pipelineType) beforeAfterTestType
		testActionProviderType      = func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType
	)

	logger := loggerProvider()

	getPipeline := func(name string, state dstate.Cache) (PreparedDefaultPipelineProvider, *phasedExecutionContext[contextType]) {
		ctx := NewDefaultPhasedExecutionContext(
			OperationDestroy,
			func(data OnPhaseFuncData[contextType]) error {
				return nil
			},
			func(progress Progress) error {
				return nil
			},
		)

		return func() Pipeline[DefaultContextType] {
			return NewPipelineWithStateCache(ctx, state, loggerProviderOpt, pipelineNameOpt(name))
		}, ctx
	}

	emptyBeforeAfter := func(testName string, state dstate.Cache, pipeline pipelineType) func() error {
		return func() error {
			logger.LogInfoF("before/after empty run: %s\n", testName)
			return nil
		}
	}

	const (
		testKey = "test"
		testVal = "yes"
	)

	writeTestState := func(state dstate.Cache) error {
		return state.Save(testKey, []byte(testVal))
	}

	getDhctlStateWithTest := func() DhctlState {
		return map[string][]byte{testKey: []byte(testVal)}
	}

	actionWithTestState := func(testName string, state dstate.Cache, msg string) actionType {
		return func(switcher switcherType) error {
			logger.LogInfoF("%s: %s\n", testName, msg)
			return writeTestState(state)
		}
	}

	actionErr := errors.New("action error")

	tests := []struct {
		name string

		afterRun      afterTestActionProviderType
		afterRunError error

		actionProvider testActionProviderType
		actionErr      error

		expectedState DhctlState
	}{
		{
			name: "pipeline already started",

			afterRun:      emptyBeforeAfter,
			afterRunError: nil,

			actionProvider: func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType {
				return func(switcher switcherType) error {
					logger.LogInfoF("action run: %s\n", testName)
					return pipeline.Run(actionWithTestState(testName, state, "pipeline already started"))
				}
			},
			actionErr: ErrPipelineAlreadyStarted,

			expectedState: nil,
		},

		{
			name: "double start pipeline",

			afterRun: func(testName string, state dstate.Cache, pipeline pipelineType) beforeAfterTestType {
				return func() error {
					logger.LogInfoF("after run: %s\n", testName)
					return pipeline.Run(actionWithTestState(testName, state, "double start"))
				}
			},
			afterRunError: ErrPipelineAlreadyFinished,

			actionProvider: func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType {
				return func(switcher switcherType) error {
					logger.LogInfoF("action run: %s\n", testName)
					return nil
				}
			},
			actionErr: nil,

			expectedState: nil,
		},

		{
			name: "action returns error",

			afterRun:      emptyBeforeAfter,
			afterRunError: nil,

			actionProvider: func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType {
				return func(switcher switcherType) error {
					logger.LogInfoF("action returns error: %s\n", testName)
					if err := writeTestState(state); err != nil {
						return err
					}

					return actionErr
				}
			},
			actionErr: actionErr,

			expectedState: getDhctlStateWithTest(),
		},

		{
			name: "action succeeds",

			afterRun:      emptyBeforeAfter,
			afterRunError: nil,

			actionProvider: func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType {
				return func(switcher switcherType) error {
					logger.LogInfoF("action succeed: %s\n", testName)
					if err := writeTestState(state); err != nil {
						return err
					}

					return nil
				}
			},
			actionErr: nil,

			expectedState: getDhctlStateWithTest(),
		},

		{
			name: "switch state save last state",

			afterRun:      emptyBeforeAfter,
			afterRunError: nil,

			actionProvider: func(t *testing.T, testName string, state dstate.Cache, pipeline pipelineType) actionType {
				return func(switcher switcherType) error {
					require.Equal(t, DhctlState(nil), pipeline.GetLastState())

					logger.LogInfoF("switch state: %s\n", testName)
					if err := writeTestState(state); err != nil {
						return err
					}

					if err := switcher(WaitStaticDestroyerNodeUserPhase, false, nil); err != nil {
						return err
					}

					require.Equal(t, getDhctlStateWithTest(), pipeline.GetLastState())

					return nil
				}
			},
			actionErr: nil,

			expectedState: getDhctlStateWithTest(),
		},
	}

	assertTypedError := func(t *testing.T, err error, expectedErr error, msg string) {
		if expectedErr == nil {
			require.NoError(t, err)
			return
		}
		require.Error(t, err, msg, expectedErr)
		require.True(t, errors.Is(err, expectedErr), err.Error())
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := cache.NewTestCache()
			pipelineProvider, _ := getPipeline(tt.name, state)
			pipeline := pipelineProvider()

			action := tt.actionProvider(t, tt.name, state, pipeline)
			err := pipeline.Run(action)
			assertTypedError(t, err, tt.actionErr, "action")

			err = tt.afterRun(tt.name, state, pipeline)()
			assertTypedError(t, err, tt.afterRunError, "after run")

			require.Equal(t, tt.expectedState, pipeline.GetLastState())
		})
	}

	t.Run("get action without start pipeline", func(t *testing.T) {
		state := cache.NewTestCache()
		pipelineProvider, _ := getPipeline("should stop returns nil and not continue", state)
		pipeline := pipelineProvider()

		actionNotStart := pipeline.ActionInPipeline()

		notChanged := true

		err := actionNotStart.Run(WaitStaticDestroyerNodeUserPhase, false, func() (contextType, error) {
			logger.LogInfoLn("Start actionNotStart never printed")

			err := state.Save("not saved", []byte("yes"))
			require.NoError(t, err)

			notChanged = false

			return nil, nil
		})

		require.Error(t, err, "action not started")
		require.True(t, errors.Is(err, ErrPipelineDidNotStart))
		require.True(t, notChanged)
		require.Equal(t, DhctlState(nil), pipeline.GetLastState())
	})

	t.Run("action phase switcher returns should stop", func(t *testing.T) {
		state := cache.NewTestCache()
		pipelineProvider, ctx := getPipeline("action phase switcher returns should stop", state)
		pipeline := pipelineProvider()

		notChanged := true
		var switcherErr error

		err := pipeline.Run(func(switcher switcherType) error {
			logger.LogInfoLn("Start pipeline phase switcher returns should stop")
			if err := writeTestState(state); err != nil {
				return err
			}

			logger.LogInfoLn("Set should stop")
			ctx.stopOperationCondition = true

			if err := switcher(WaitStaticDestroyerNodeUserPhase, false, nil); err != nil {
				switcherErr = err
				return err
			}

			actionNotStart := pipeline.ActionInPipeline()
			return actionNotStart.Run(DeleteResourcesPhase, false, func() (contextType, error) {
				logger.LogInfoLn("Start actionNotStart never printed")
				notChanged = false
				return nil, state.Save("not saved", []byte("yes"))
			})
		})

		require.NoError(t, err, "pipeline should be stopped")
		require.True(t, errors.Is(switcherErr, ErrShouldStop))
		require.True(t, notChanged)
		require.Equal(t, DhctlState(nil), pipeline.GetLastState())
	})

	t.Run("should stop returns nil and not continue", func(t *testing.T) {
		state := cache.NewTestCache()
		pipelineProvider, ctx := getPipeline("should stop returns nil and not continue", state)
		pipeline := pipelineProvider()

		err := pipeline.Run(func(switcher switcherType) error {
			logger.LogInfoLn("Start pipeline should stop action")
			actionWithState := pipeline.ActionInPipeline()

			err := actionWithState.Run(CreateStaticDestroyerNodeUserPhase, false, func() (contextType, error) {
				logger.LogInfoLn("Start actionWithState")
				return nil, writeTestState(state)
			})
			require.NoError(t, err)
			require.Equal(t, getDhctlStateWithTest(), pipeline.GetLastState())

			logger.LogInfoLn("Set should stop")
			ctx.stopOperationCondition = true

			actionShouldStop := pipeline.ActionInPipeline()

			notChanged := true

			err = actionShouldStop.Run(WaitStaticDestroyerNodeUserPhase, false, func() (contextType, error) {
				logger.LogInfoLn("Start actionShouldStop never printed")

				err := state.Save("not saved", []byte("yes"))
				require.NoError(t, err)

				notChanged = false

				return nil, nil
			})
			require.True(t, errors.Is(err, ErrShouldStop))
			require.True(t, notChanged)
			require.Equal(t, getDhctlStateWithTest(), pipeline.GetLastState())

			return err
		})

		require.NoError(t, err)
	})

	t.Run("dummy pipeline follow interface", func(t *testing.T) {
		state := cache.NewTestCache()
		pipelineProvider := NewDummyDefaultPipelineProviderOpts(loggerProviderOpt, WithPipelineName("dummy pipeline does not panic"))
		pipeline := pipelineProvider()

		errorAction := pipeline.ActionInPipeline()

		notChanged := true
		err := errorAction.Run(CreateStaticDestroyerNodeUserPhase, false, func() (contextType, error) {
			logger.LogInfoLn("Does not print")

			notChanged = false
			err := state.Save("not saved", []byte("yes"))
			require.NoError(t, err)

			return nil, nil
		})

		require.True(t, errors.Is(err, ErrPipelineDidNotStart), "dummy pipeline should follow interface")
		require.True(t, notChanged)
		require.Equal(t, DhctlState(nil), pipeline.GetLastState())
		notSaved, err := state.InCache("not saved")
		require.NoError(t, err)
		require.False(t, notSaved)

		err = pipeline.Run(func(switcher switcherType) error {
			logger.LogInfoLn("Start dummy pipeline action")
			actionWithState := pipeline.ActionInPipeline()

			err := actionWithState.Run(CreateStaticDestroyerNodeUserPhase, false, func() (contextType, error) {
				logger.LogInfoLn("Start actionWithState")
				return nil, writeTestState(state)
			})
			require.NoError(t, err)
			require.Equal(t, DhctlState(nil), pipeline.GetLastState())
			inCacheState, err := ExtractDhctlState(state)
			require.NoError(t, err)
			require.Equal(t, getDhctlStateWithTest(), inCacheState)

			err = switcher(DeleteResourcesPhase, false, nil)
			require.NoError(t, err, "dummy switcher does not panic")

			return nil
		})

		require.NoError(t, err)

		notChanged = true
		err = pipeline.Run(func(switcher switcherType) error {
			logger.LogInfoLn("Should not printed")
			notChanged = false
			return nil
		})

		require.True(t, errors.Is(err, ErrPipelineAlreadyFinished), "dummy pipeline should follow interface")
		require.True(t, notChanged)
	})
}
