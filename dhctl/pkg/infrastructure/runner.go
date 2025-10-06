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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
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

	hasChangesExitCode = 2

	infrastructurePipelineAbortedMessage = `
Infrastructure pipeline aborted.
If you want to drop the cache and continue, please run dhctl with "--yes-i-want-to-drop-cache" flag.
`
)

const (
	PlanHasNoChanges = iota
	PlanHasChanges
	PlanHasDestructiveChanges
)

var (
	ErrRunnerStopped              = errors.New("Infrastructure runner was stopped.")
	ErrInfrastructureApplyAborted = errors.New("Infrastructure apply aborted.")
)

type (
	ExecutorProvider func(string, log.Logger) Executor
	StateChecker     func([]byte) error
)

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
	name       string
	prefix     string
	step       string
	workingDir string

	statePath     string
	planPath      string
	variablesPath string

	changeSettings ChangeActionSettings

	allowedCachedState     bool
	changesInPlan          int
	planDestructiveChanges *PlanDestructiveChanges
	hasMasterDestruction   bool
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
	infraExecutor                       Executor
	infraExecutorProvider               ExecutorProvider

	hook InfraActionHook

	vmTypeMu  sync.Mutex
	vmTypeMap map[string]string
}

func NewRunner(cfg *config.MetaConfig, step string, stateCache state.Cache, executorProvider ExecutorProvider) *Runner {
	provider := cfg.ProviderName
	prefix := cfg.ClusterPrefix
	layout := cfg.Layout

	workingDir := infrastructure.GetInfrastructureModulesForRunningDir(provider, layout, step)
	logger := log.GetDefaultLogger()

	// in terraform >= 0.14 creates special file contains hashes for used providers between runs terraform
	// it needs for prevent using different providers versions for same infrastructure
	// because user can use constraints for definition provider version and terraform can upgrade version without
	// user permission. Now if user get conflict between versions user should confirm upgrade with command
	// terraform init -upgrade
	// https://developer.hashicorp.com/terraform/tutorials/configuration-language/provider-versioning
	// unfortunately we can use different provider version between runs. For example cloud provider vcd works
	// in two modes in legacy (for old vcd instances (yes, some our customers cannot upgrade vcd version)) and latest.
	// user can switch from and to legacy mode in between dhctl runs in same container (in commander for example)
	// and in this situation user will get provider lock error.
	// we made a decision that we will remove lock file before infrastructure running
	err := releaseInfrastructureProviderLock(infrastructure.GetDhctlPath(), infrastructure.GetInfrastructureModulesDir(provider), step, workingDir, logger)
	if err != nil {
		// yes, we panic here because returns error in many places and this will lead to a major refactoring, and
		// we don't expect any problems with file deletion, only in critical situations
		panic(fmt.Errorf("failed to release infrastructure lock: %v", err))
	}

	r := &Runner{
		prefix:                prefix,
		step:                  step,
		name:                  step,
		workingDir:            workingDir,
		confirm:               input.NewConfirmation,
		stateCache:            stateCache,
		changeSettings:        ChangeActionSettings{},
		infraExecutor:         executorProvider(workingDir, logger),
		logger:                logger,
		infraExecutorProvider: executorProvider,
	}

	var destinations []SaverDestination
	cacheDest := getCacheDestination(r)
	if cacheDest != nil {
		destinations = []SaverDestination{cacheDest}
	}

	r.stateSaver = NewStateSaver(destinations)
	return r
}

func NewRunnerFromConfig(cfg *config.MetaConfig, step string, stateCache state.Cache, executorProvider ExecutorProvider) *Runner {
	return NewRunner(cfg, step, stateCache, executorProvider)
}

func NewImmutableRunnerFromConfig(cfg *config.MetaConfig, step string, executorProvider ExecutorProvider) *Runner {
	return NewRunner(cfg, step, cache.Dummy(), executorProvider)
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

func (r *Runner) WorkerDir() string {
	return r.workingDir
}

func (r *Runner) GetExecutorProvider() ExecutorProvider {
	return r.infraExecutorProvider
}

func (r *Runner) WithState(stateData []byte) *Runner {
	tmpFile, err := os.CreateTemp(app.TmpDirName, r.step+deckhouseClusterStateSuffix)
	if err != nil {
		log.ErrorF("can't save infrastructure state for runner %s: %s\n", r.step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), stateData, 0o600)
	if err != nil {
		log.ErrorF("can't write infrastructure state for runner %s: %s\n", r.step, err)
		return r
	}

	r.statePath = tmpFile.Name()
	return r
}

func (r *Runner) WithVariables(variablesData []byte) *Runner {
	tmpFile, err := os.CreateTemp(app.TmpDirName, varFileName)
	if err != nil {
		log.ErrorF("can't save infrastructure variables for runner %s: %s\n", r.step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), variablesData, 0o600)
	if err != nil {
		log.ErrorF("can't write infrastructure variables for runner %s: %s\n", r.step, err)
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
					err := fmt.Errorf("can't write infrastructure state for runner %s: %s", r.step, err)
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
			err := r.infraExecutor.Init(ctx, fmt.Sprintf("%s/plugins", strings.TrimRight(infrastructure.GetDhctlPath(), "/")))
			return 0, err
		})

		return err
	})
}

func (r *Runner) stateName() string {
	return fmt.Sprintf("%s.tfstate", r.name)
}

func (r *Runner) getHook() InfraActionHook {
	if r.hook == nil {
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
	if r.changesInPlan == PlanHasDestructiveChanges && r.changeSettings.AutoDismissDestructive {
		// skip plan
		return true, nil
	}

	if r.changesInPlan == PlanHasNoChanges {
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

		if r.stateChecker != nil {
			err = r.logger.LogProcess("default", "infrastructure state check before apply...", func() error {
				if r.statePath == "" {
					log.InfoF("Infrastructure state path is empty. Skip infrastructure state check.")
					return nil
				}

				st, err := os.ReadFile(r.statePath)
				if err != nil {
					if os.IsNotExist(err) {
						log.DebugF("File %s with state not found, Probably call apply with new resource. Skip check.", r.statePath)
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

func (r *Runner) Plan(ctx context.Context, destroy bool) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return r.logger.LogProcess("default", "infrastructure plan ...", func() error {
		tmpFile, err := os.CreateTemp(app.TmpDirName, r.step+deckhousePlanSuffix)
		if err != nil {
			return fmt.Errorf("can't create temp file for plan: %w", err)
		}

		exitCode, err := r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			return r.infraExecutor.Plan(ctx, PlanOpts{
				StatePath:        r.statePath,
				Destroy:          destroy,
				VariablesPath:    r.variablesPath,
				OutPath:          tmpFile.Name(),
				DetailedExitCode: true,
			})
		})

		if exitCode == hasChangesExitCode {
			r.changesInPlan = PlanHasChanges
			report, err := r.getPlanDestructiveChanges(ctx, tmpFile.Name())
			destructiveChanges := report.Changes
			if err != nil {
				return err
			}
			if destructiveChanges != nil {
				r.changesInPlan = PlanHasDestructiveChanges
				r.planDestructiveChanges = destructiveChanges
				r.hasMasterDestruction = report.hasMasterDestruction
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
		return nil, fmt.Errorf("no state found, try to run infastructure apply first")
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
		return nil, fmt.Errorf("can't get infrastructure output for %q\n%v", output, err)
	}

	return result, nil
}

func (r *Runner) Destroy(ctx context.Context) error {
	if r.stopped {
		return ErrRunnerStopped
	}

	if r.statePath == "" {
		return fmt.Errorf("no state found, try to run infrastructure apply first")
	}

	if r.changeSettings.AutoDismissChanges {
		return ErrInfrastructureApplyAborted
	}

	if r.changeSettings.AutoDismissDestructive {
		r.logger.LogInfoLn("infrastructure destroy skipped")
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
			return fmt.Errorf("infrastructure destroy aborted")
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

func (r *Runner) GetStep() string {
	return r.step
}

func (r *Runner) GetChangesInPlan() int {
	return r.changesInPlan
}

func (r *Runner) GetMasterDestruction() bool {
	return r.hasMasterDestruction
}

func (r *Runner) GetPlanDestructiveChanges() *PlanDestructiveChanges {
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
	r.logger.LogInfoF("Infrastructure runner %q process exited.\n", r.step)

	return exitCode, err
}

func (r *Runner) getPlanDestructiveChanges(ctx context.Context, planFile string) (*DestructiveChangesReport, error) {
	var result []byte
	var providerName string
	var hasMasterInstanceDestructiveChanges bool

	resTypeMap, err := r.getProviderVMTypes()
	if err != nil {
		return nil, err
	}

	_, err = r.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
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
		return nil, fmt.Errorf("can't get infrastructure plan for %q\n%v", planFile, err)
	}

	var plan TfPlan
	if err := json.Unmarshal(result, &plan); err != nil {
		return nil, err
	}

	var destructiveChanges *PlanDestructiveChanges
	getOrCreateDestructiveChanges := func() *PlanDestructiveChanges {
		if destructiveChanges == nil {
			destructiveChanges = &PlanDestructiveChanges{}
		}
		return destructiveChanges
	}

	for _, resource := range plan.ResourceChanges {
		if hasAction(resource.Change.Actions, "delete") {
			if providerName == "" && resource.ProviderName != "" {
				providerName = resource.ProviderName
			}
			if !hasMasterInstanceDestructiveChanges {
				hasMasterInstanceDestructiveChanges = IsMasterInstanceDestructiveChanged(ctx, resource, resTypeMap)
			}
			if hasAction(resource.Change.Actions, "create") {
				// recreate
				getOrCreateDestructiveChanges().ResourcesRecreated = append(getOrCreateDestructiveChanges().ResourcesRecreated, ValueChange{
					CurrentValue: resource.Change.Before,
					NextValue:    resource.Change.After,
					Type:         resource.Type,
				})
			} else {
				getOrCreateDestructiveChanges().ResourcesDeleted = append(getOrCreateDestructiveChanges().ResourcesDeleted, ValueChange{
					CurrentValue: resource.Change.Before,
					Type:         resource.Type,
				})
			}
		}
	}
	log.DebugF("hasMasterDestruction: %s\n", hasMasterInstanceDestructiveChanges)
	return &DestructiveChangesReport{
		Changes:              destructiveChanges,
		hasMasterDestruction: hasMasterInstanceDestructiveChanges,
	}, nil
}

func hasAction(actions []string, findAction string) bool {
	for _, action := range actions {
		if action == findAction {
			return true
		}
	}
	return false
}

func deleteLockFile(fileForDelete, logPrefix, nextActionLogString string, logger log.Logger) (pursue bool, err error) {
	err = os.Remove(fileForDelete)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}

		logger.LogDebugF("%s file %s not found. %s\n", logPrefix, fileForDelete, nextActionLogString)
		return true, nil
	}

	logger.LogDebugF("%s file %s was found and deleted. \n", logPrefix, fileForDelete)
	return false, nil
}

func releaseInfrastructureProviderLock(dhctlDir, modulesDir, module, desiredModuleDir string, logger log.Logger) error {
	logger.LogDebugF("Releasing infrastructure provider lock. dhctl dir: %s; modules dir %s; module: %s; desired module dir: %s .\n",
		dhctlDir, modulesDir, module, desiredModuleDir)
	defer logger.LogDebugF("Releasing infrastructure provider lock finished.\n")

	// terraform and tofu use same file name for lock file
	const lockFile = ".terraform.lock.hcl"

	// first, we will process terraform case. Terraform 0.14 version save lock file in same location where terraform runs
	terraformLockFile := filepath.Join(dhctlDir, lockFile)
	logger.LogDebugF("Terraform lock file %s\n", terraformLockFile)

	_, err := deleteLockFile(terraformLockFile, "Terraform lock", "", logger)
	if err != nil {
		return err
	}

	// we need to continue processing for tofu because commander can work in next sequence
	// - converge tofu cluster
	// - converge terraform cluster
	// - converge tofu cluster
	// in this case we release lock from terraform because terraform lock was present, but tofu lock presents from
	// first run also present and is not deleted. So, we should continue to delete tofu locks in all cases
	log.DebugLn("Try to delete tofu lock files regardless of existing terraform lock.")

	// next, we will process tofu case. Latest terraform version and opentofu can save lock in modules dir (not in desired
	// module where tofu will run) and in desired module. I do not understand because this behavior happens

	tofuModulesLockFile := filepath.Join(modulesDir, module, lockFile)
	logger.LogDebugF("Tofu modules lock file %s\n", tofuModulesLockFile)

	pursue, err := deleteLockFile(tofuModulesLockFile, "Tofu modules lock", "Try to delete tofu lock file in module.", logger)
	if err != nil {
		return err
	}

	if !pursue {
		logger.LogDebugF("Tofu modules lock file %s was deleted. Hence we do not need delete tofu 'in module' lock file.\n", tofuModulesLockFile)
		return nil
	}

	tofuInModuleLockFile := filepath.Join(desiredModuleDir, lockFile)
	logger.LogDebugF("Tofu 'in module' lock file %s\n", tofuInModuleLockFile)

	_, err = deleteLockFile(tofuInModuleLockFile, "Tofu 'in module' lock", "", logger)
	if err != nil {
		return err
	}

	return nil
}
