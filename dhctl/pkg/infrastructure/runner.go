// Copyright 2021 Flant JSC
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	deckhouseClusterStateSuffix = "-dhctl.*.tfstate"
	deckhousePlanSuffix         = "-dhctl.*.tfplan"
	varFileName                 = "cluster-config.auto.*.tfvars.json"

	infrastructurePipelineAbortedMessage = `
Infrastructure pipeline aborted.
If you want to drop the cache and continue, please run dhctl with "--yes-i-want-to-drop-cache" flag.
`
)

var (
	ErrRunnerStopped              = errors.New("Infrastructure runner was stopped.")
	ErrInfrastructureApplyAborted = errors.New("Infrastructure apply aborted.")
)

type (
	StateChecker func([]byte) error
)

type destructiveChangesReport struct {
	changes      *plan.DestructiveChanges
	hasVMChanges bool
}

type AutoApproveSettings struct {
	AutoApprove bool
}

type AutomaticSettings struct {
	AutoApproveSettings

	AutoDismissChanges     bool
	AutoDismissDestructive bool
}

type ChangeActionSettings struct {
	AutomaticSettings
	SkipChangesOnDeny bool
}

type Runner struct {
	name   string
	prefix string

	statePath     string
	planPath      string
	variablesPath string

	changeSettings ChangeActionSettings

	allowedCachedState     bool
	changesInPlan          int
	planDestructiveChanges *plan.DestructiveChanges
	hasVMDestruction       bool
	stateCache             state.Cache

	stateSaver   *StateSaver
	stateChecker StateChecker

	confirm func() *input.Confirmation
	stopped bool

	logger log.Logger

	// Atomic flag to check weather infrastructure utility is running. Do not manually change its values.
	// Odd number - infrastructure utility is running
	// Even number - runner is in standby mode
	infrastructureUtilityRunningCounter int32

	infraExecutor Executor

	hook InfraActionHook
}

func NewRunner(cfg *config.MetaConfig, stateCache state.Cache, executor Executor) *Runner {
	prefix := cfg.ClusterPrefix

	logger := log.GetDefaultLogger()

	step := executor.Step()

	r := &Runner{
		prefix:         prefix,
		name:           string(step),
		confirm:        input.NewConfirmation,
		stateCache:     stateCache,
		changeSettings: ChangeActionSettings{},
		infraExecutor:  executor,
		logger:         logger,
	}

	var destinations []SaverDestination
	cacheDest := getCacheDestination(r)
	if cacheDest != nil {
		destinations = []SaverDestination{cacheDest}
	}

	r.stateSaver = NewStateSaver(destinations)
	return r
}

func NewRunnerFromConfig(cfg *config.MetaConfig, stateCache state.Cache, executor Executor) *Runner {
	return NewRunner(cfg, stateCache, executor)
}

func NewImmutableRunnerFromConfig(cfg *config.MetaConfig, executor Executor) *Runner {
	return NewRunner(cfg, cache.Dummy(), executor)
}

func (r *Runner) WithCache(cache state.Cache) *Runner {
	r.stateCache = cache
	return r
}

func (r *Runner) WithName(name string) *Runner {
	r.name = name
	return r
}

func (r *Runner) WithConfirm(confirm func() *input.Confirmation) *Runner {
	r.confirm = confirm
	return r
}

func (r *Runner) WithStatePath(statePath string) *Runner {
	r.statePath = statePath
	return r
}

func (r *Runner) WithStateChecker(checker StateChecker) *Runner {
	r.stateChecker = checker
	return r
}

func (r *Runner) WithHook(h InfraActionHook) *Runner {
	r.hook = h
	return r
}

func (r *Runner) WithState(stateData []byte) *Runner {
	step := r.infraExecutor.Step()

	tmpFile, err := os.CreateTemp(r.infraExecutor.GetStatesDir(), string(step)+deckhouseClusterStateSuffix)
	if err != nil {
		log.ErrorF("Can't save infrastructure state for runner %s: %s\n", step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), stateData, 0o600)
	if err != nil {
		log.ErrorF("Can't write infrastructure state for runner %s: %s\n", step, err)
		return r
	}

	r.statePath = tmpFile.Name()
	return r
}

func (r *Runner) WithVariables(variablesData []byte) *Runner {
	step := r.infraExecutor.Step()

	tmpFile, err := os.CreateTemp(r.infraExecutor.GetStatesDir(), varFileName)
	if err != nil {
		log.ErrorF("Can't save infrastructure variables for runner %s: %s\n", step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), variablesData, 0o600)
	if err != nil {
		log.ErrorF("Can't write infrastructure variables for runner %s: %s\n", step, err)
		return r
	}

	r.variablesPath = tmpFile.Name()
	return r
}

func (r *Runner) WithAutoApprove(flag bool) *Runner {
	r.changeSettings.AutoApprove = flag
	return r
}

func (r *Runner) WithAutoDismissDestructiveChanges(flag bool) *Runner {
	r.changeSettings.AutoDismissDestructive = flag
	return r
}

func (r *Runner) WithAutoDismissChanges(flag bool) *Runner {
	r.changeSettings.AutoDismissChanges = flag
	return r
}

func (r *Runner) WithAllowedCachedState(flag bool) *Runner {
	r.allowedCachedState = flag
	return r
}

func (r *Runner) WithSkipChangesOnDeny(flag bool) *Runner {
	r.changeSettings.SkipChangesOnDeny = flag
	return r
}

// WithAdditionalStateSaverDestination
// by default we use intermediate save state to cache destination
func (r *Runner) WithAdditionalStateSaverDestination(destinations ...SaverDestination) *Runner {
	r.stateSaver.addDestinations(destinations...)
	return r
}

func (r *Runner) WithSingleShotMode(enabled bool) RunnerInterface {
	if enabled {
		return NewSingleShotRunner(r)
	}
	return r
}

func (r *Runner) WithLogger(logger log.Logger) *Runner {
	r.logger = logger
	return r
}

func (r *Runner) GetLogger() log.Logger {
	return r.logger
}

func (r *Runner) switchInfrastructureUtilityIsRunning() {
	atomic.AddInt32(&r.infrastructureUtilityRunningCounter, 1)
}

func (r *Runner) checkInfrastructureUtilityIsRunning() bool {
	return (atomic.LoadInt32(&r.infrastructureUtilityRunningCounter) % 2) > 0
}

func (r *Runner) Init(ctx context.Context) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	if r.statePath == "" {
		// Save state directly in the cache to prevent state loss
		stateName := r.stateName()
		r.statePath = r.stateCache.GetPath(stateName)

		hasState, err := r.stateCache.InCache(stateName)
		if err != nil {
			return err
		}

		if hasState {
			r.logger.LogInfoF("Cached infrastructure state found:\n\t%s\n\n", r.statePath)
			if !r.allowedCachedState {
				var isConfirm bool
				switch app.UseTfCache {
				case app.UseStateCacheYes:
					isConfirm = true
				case app.UseStateCacheNo:
					isConfirm = false
				default:
					isConfirm = r.confirm().
						WithMessage("Do you want to continue with infrastructure state from local cache?").
						WithYesByDefault().
						Ask()
				}

				if !isConfirm {
					return fmt.Errorf(infrastructurePipelineAbortedMessage)
				}
			}

			stateData, err := r.stateCache.Load(stateName)
			if err != nil {
				return err
			}

			if len(stateData) > 0 {
				err := fs.WriteContentIfNeed(r.statePath, stateData)
				if err != nil {
					err := fmt.Errorf("Can't write infrastructure state for runner %s: %s", r.infraExecutor.Step(), err)
					r.logger.LogErrorLn(err)
					return err
				}
			}
		}
	}

	// If statePath still empty, it means that something wrong with cache. Let's create file for the state in tmp directory.
	if r.statePath == "" {
		r.WithState(nil)
	}

	return r.logger.LogProcess("default", "infrastructure init ...", func() error {
		_, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			err := r.infraExecutor.Init(ctx)
			return 0, err
		})

		return err
	})
}

func (r *Runner) stateName() string {
	return fmt.Sprintf("%s.tfstate", r.name)
}

func (r *Runner) getHook() InfraActionHook {
	if govalue.IsNil(r.hook) {
		return &DummyHook{}
	}

	return r.hook
}

func (r *Runner) runBeforeActionAndWaitReady(ctx context.Context) error {
	hook := r.getHook()

	runPostAction, err := hook.BeforeAction(ctx, r)
	if err != nil {
		return err
	}

	if err := hook.IsReady(); err != nil {
		var resErr *multierror.Error
		resErr = multierror.Append(resErr, err)

		if runPostAction {
			err := hook.AfterAction(ctx, r)
			if err != nil {
				resErr = multierror.Append(resErr, err)
			}
		}

		return resErr.ErrorOrNil()
	}

	return nil
}

func (r *Runner) isSkipChanges(ctx context.Context) (skip bool, err error) {
	// first verify destructive change
	if r.changesInPlan == plan.HasDestructiveChanges && r.changeSettings.AutoDismissDestructive {
		// skip plan
		return true, nil
	}

	if r.changesInPlan == plan.HasNoChanges {
		// if plan has not changes we will run apply
		return false, nil
	}

	if r.changeSettings.AutoDismissChanges {
		return false, ErrInfrastructureApplyAborted
	}

	if !r.changeSettings.AutoApprove {
		if !r.confirm().WithMessage("Do you want to CHANGE objects state in the cloud?").Ask() {
			if r.changeSettings.SkipChangesOnDeny {
				return true, nil
			}
			return false, ErrInfrastructureApplyAborted
		}
	}

	err = r.runBeforeActionAndWaitReady(ctx)

	return false, err
}

func (r *Runner) Apply(ctx context.Context) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return r.logger.LogProcess("default", "infrastructure apply ...", func() error {
		skip, err := r.isSkipChanges(ctx)
		if err != nil {
			return err
		}
		if skip {
			r.logger.LogInfoLn("Skip infrastructure apply.")
			return nil
		}

		if !govalue.IsNil(r.stateChecker) {
			err = r.logger.LogProcess("default", "infrastructure state check before apply...", func() error {
				if r.statePath == "" {
					log.InfoF("Infrastructure state path is empty. Skip infrastructure state check.\n")
					return nil
				}

				st, err := os.ReadFile(r.statePath)
				if err != nil {
					if os.IsNotExist(err) {
						log.DebugF("File %s with state not found, Probably call apply with new resource. Skip check.\n", r.statePath)
						return nil
					}
					return err
				}

				return r.stateChecker(st)
			})
			if err != nil {
				return err
			}
		}

		err = r.stateSaver.Start(r)
		if err != nil {
			return err
		}
		defer r.stateSaver.Stop()

		_, err = r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			err := r.infraExecutor.Apply(ctx, ApplyOpts{
				StatePath:     r.statePath,
				PlanPath:      r.planPath,
				VariablesPath: r.variablesPath,
			})
			return 0, err
		})

		var errRes *multierror.Error
		errRes = multierror.Append(errRes, err)

		// yes, do not check err from exec infra utility
		// always run post action if need
		err = r.getHook().AfterAction(ctx, r)
		errRes = multierror.Append(errRes, err)

		return errRes.ErrorOrNil()
	})
}

func (r *Runner) ShowPlan(ctx context.Context) ([]byte, error) {
	if r.stopped {
		return nil, ErrRunnerStopped
	}

	rawPlan, err := r.infraExecutor.Show(ctx, r.GetPlanPath())
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("Can't get infrastructure plan for %q\n%v", r.GetPlanPath(), err)
	}

	return rawPlan, nil
}

func (r *Runner) Plan(ctx context.Context, destroy, noout bool) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return r.logger.LogProcess("default", "infrastructure plan ...", func() error {
		tmpFile, err := os.CreateTemp(r.infraExecutor.GetStatesDir(), string(r.infraExecutor.Step())+deckhousePlanSuffix)
		if err != nil {
			return fmt.Errorf("Can't create temp file for plan: %w", err)
		}

		exitCode, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			return r.infraExecutor.Plan(ctx, PlanOpts{
				StatePath:        r.statePath,
				Destroy:          destroy,
				VariablesPath:    r.variablesPath,
				OutPath:          tmpFile.Name(),
				DetailedExitCode: true,
				NoOutput:         noout,
			})
		})

		// todo need refactor
		if exitCode == infraexec.HasChangesExitCode {
			r.changesInPlan = plan.HasChanges
			if noout {
				destructiveChanged, err := r.planHasDestructiveChanges(ctx, tmpFile.Name())
				if err != nil {
					return err
				}
				if destructiveChanged {
					r.changesInPlan = plan.HasDestructiveChanges
				}
			} else {
				report, err := r.getPlanDestructiveChanges(ctx, tmpFile.Name())
				destructiveChanges := report.changes
				if err != nil {
					return err
				}
				if destructiveChanges != nil {
					r.changesInPlan = plan.HasDestructiveChanges
					r.planDestructiveChanges = destructiveChanges
					r.hasVMDestruction = report.hasVMChanges
				}
			}
		} else if err != nil {
			return err
		}

		r.planPath = tmpFile.Name()

		return nil
	})
}

func (r *Runner) GetInfrastructureOutput(ctx context.Context, output string) ([]byte, error) {
	if r.stopped {
		return nil, ErrRunnerStopped
	}

	if r.statePath == "" {
		return nil, fmt.Errorf("No state found, try to run infastructure apply first")
	}

	var result []byte

	_, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
		res, err := r.infraExecutor.Output(ctx, r.statePath, []string{output}...)
		if err != nil {
			return -1, err
		}

		result = res
		return 0, nil
	})
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("Can't get infrastructure output for %q\n%v", output, err)
	}

	return result, nil
}

func (r *Runner) Destroy(ctx context.Context) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	if r.statePath == "" {
		return fmt.Errorf("No state found, try to run infrastructure apply first")
	}

	if r.changeSettings.AutoDismissChanges {
		return ErrInfrastructureApplyAborted
	}

	if r.changeSettings.AutoDismissDestructive {
		r.logger.LogInfoLn("Infrastructure destroy skipped.")
		return nil
	}

	_, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
		_, err := r.infraExecutor.Plan(ctx, PlanOpts{
			Destroy:       true,
			StatePath:     r.statePath,
			VariablesPath: r.variablesPath,
		})

		return 0, err
	})
	if err != nil {
		return fmt.Errorf("Cannot prepare terrafrom destroy plan: %w", err)
	}

	if !r.changeSettings.AutoApprove {
		if !r.confirm().WithMessage("Do you want to DELETE objects from the cloud?").Ask() {
			return fmt.Errorf("Infrastructure destroy aborted.")
		}
	}

	err = r.runBeforeActionAndWaitReady(ctx)
	if err != nil {
		return err
	}

	return r.logger.LogProcess("default", "infrastructure destroy ...", func() error {
		err := r.stateSaver.Start(r)
		if err != nil {
			return err
		}
		defer r.stateSaver.Stop()

		_, err = r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			err := r.infraExecutor.Destroy(ctx, DestroyOpts{
				StatePath:     r.statePath,
				VariablesPath: r.variablesPath,
			})

			return 0, err
		})

		var errRes *multierror.Error
		errRes = multierror.Append(errRes, err)

		// yes, do not check err from exec infra utility
		// always run post action if need
		err = r.getHook().AfterAction(ctx, r)
		errRes = multierror.Append(errRes, err)

		return errRes.ErrorOrNil()
	})
}

func (r *Runner) ResourcesQuantityInState() int {
	if r.statePath == "" {
		return 0
	}

	data, err := os.ReadFile(r.statePath)
	if err != nil {
		r.logger.LogErrorLn(err)
		return 0
	}

	var st struct {
		Resources []json.RawMessage `json:"resources"`
	}
	err = json.Unmarshal(data, &st)
	if err != nil {
		r.logger.LogErrorLn(err)
		return 0
	}

	return len(st.Resources)
}

func (r *Runner) GetState() ([]byte, error) {
	return os.ReadFile(r.statePath)
}

func (r *Runner) GetStep() Step {
	return r.infraExecutor.Step()
}

func (r *Runner) GetChangesInPlan() int {
	return r.changesInPlan
}

func (r *Runner) HasVMDestruction() bool {
	return r.hasVMDestruction
}

func (r *Runner) GetPlanDestructiveChanges() *plan.DestructiveChanges {
	return r.planDestructiveChanges
}

func (r *Runner) GetPlanPath() string {
	return r.planPath
}

// Stop interrupts the current runner command and sets
// a flag to prevent executions of next runner commands.
func (r *Runner) Stop() {
	if r.checkInfrastructureUtilityIsRunning() && !r.stopped {
		log.DebugF("Runner Stop is called for %s.\n", r.name)
		r.infraExecutor.Stop()
	}
	r.stopped = true
	// Wait until the running infra utility command stops.
	for r.checkInfrastructureUtilityIsRunning() {
		time.Sleep(50 * time.Millisecond)
	}
	// Wait until the StateSaver saves the Secret for Apply and Destroy commands.
	if r.stateSaver.IsStarted() {
		<-r.stateSaver.DoneCh()
	}
}

func (r *Runner) execInfrastructureUtility(ctx context.Context, executor func(ctx context.Context) (int, error)) (int, error) {
	if r.checkInfrastructureUtilityIsRunning() {
		return 0, fmt.Errorf("Infrastructure utility have been already executed.")
	}

	r.switchInfrastructureUtilityIsRunning()
	defer r.switchInfrastructureUtilityIsRunning()
	r.infraExecutor.SetExecutorLogger(r.logger)
	exitCode, err := executor(ctx)
	r.logger.LogInfoF("Infrastructure runner %q process exited.\n", r.infraExecutor.Step())

	return exitCode, err
}

func (r *Runner) getPlanDestructiveChanges(ctx context.Context, planFile string) (*destructiveChangesReport, error) {
	var result []byte
	var providerName string
	var hasVMChange bool

	_, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
		res, err := r.infraExecutor.Show(ctx, planFile)
		if err != nil {
			return 0, err
		}

		result = res
		return 0, nil
	})
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("Can't get infrastructure plan for %q\n%v", planFile, err)
	}

	var pl plan.InfrastructurePlan
	if err := json.Unmarshal(result, &pl); err != nil {
		return nil, err
	}

	var destructiveChanges *plan.DestructiveChanges
	getOrCreateDestructiveChanges := func() *plan.DestructiveChanges {
		if destructiveChanges == nil {
			destructiveChanges = &plan.DestructiveChanges{}
		}
		return destructiveChanges
	}

	for _, resource := range pl.ResourceChanges {
		if resource.HasAction(plan.ActionDelete) {
			if providerName == "" && resource.ProviderName != "" {
				providerName = resource.ProviderName
			}
			if !hasVMChange {
				hasVMChange = r.infraExecutor.IsVMChange(resource)
			}
			if resource.HasAction(plan.ActionCreate) {
				// recreate
				getOrCreateDestructiveChanges().ResourcesRecreated = append(getOrCreateDestructiveChanges().ResourcesRecreated, plan.ValueChange{
					CurrentValue: resource.Change.Before,
					NextValue:    resource.Change.After,
					Type:         resource.Type,
				})
			} else {
				getOrCreateDestructiveChanges().ResourcesDeleted = append(getOrCreateDestructiveChanges().ResourcesDeleted, plan.ValueChange{
					CurrentValue: resource.Change.Before,
					Type:         resource.Type,
				})
			}
		}
	}
	log.DebugF("HasVMDestruction: %v\n", hasVMChange)
	return &destructiveChangesReport{
		changes:      destructiveChanges,
		hasVMChanges: hasVMChange,
	}, nil
}

func (r *Runner) planHasDestructiveChanges(ctx context.Context, planFile string) (bool, error) {
	var result []string

	_, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
		res, err := r.infraExecutor.GetActions(ctx, planFile)
		if err != nil {
			return 0, err
		}

		result = res
		return 0, nil
	})

	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return false, fmt.Errorf("can't get infrastructure plan for %q\n%v", planFile, err)
	}

	for _, action := range result {
		if action == "delete" {
			return true, nil
		}
	}

	return false, nil
}
