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

package terraform

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	deckhouseClusterStateSuffix = "-dhctl.*.tfstate"
	deckhousePlanSuffix         = "-dhctl.*.tfplan"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.*.tfvars.json"

	terraformHasChangesExitCode = 2

	terraformPipelineAbortedMessage = `
Terraform pipeline aborted.
If you want to drop the cache and continue, please run dhctl with "--yes-i-want-to-drop-cache" flag.
`
)

const (
	PlanHasNoChanges = iota
	PlanHasChanges
	PlanHasDestructiveChanges
)

var (
	ErrRunnerStopped         = errors.New("Terraform runner was stopped.")
	ErrTerraformApplyAborted = errors.New("Terraform apply aborted.")
)

type ChangeActionSettings struct {
	AutoDismissDestructive bool
	AutoApprove            bool
	SkipChangesOnDeny      bool
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

	stateCache state.Cache

	stateSaver *StateSaver

	confirm func() *input.Confirmation
	stopped bool

	// Atomic flag to check weather terraform is running. Do not manually change its values.
	// Odd number - terraform is running
	// Even number - runner is in standby mode
	terraformRunningCounter int32
	terraformExecutor       Executor

	hook InfraActionHook
}

func NewRunner(provider, prefix, layout, step string, stateCache state.Cache) *Runner {
	r := &Runner{
		prefix:            prefix,
		step:              step,
		name:              step,
		workingDir:        buildTerraformPath(provider, layout, step),
		confirm:           input.NewConfirmation,
		stateCache:        stateCache,
		changeSettings:    ChangeActionSettings{},
		terraformExecutor: &CMDExecutor{},
	}

	var destinations []SaverDestination
	cacheDest := getCacheDestination(r)
	if cacheDest != nil {
		destinations = []SaverDestination{cacheDest}
	}

	r.stateSaver = NewStateSaver(destinations)
	return r
}

func NewRunnerFromConfig(cfg *config.MetaConfig, step string, stateCache state.Cache) *Runner {
	return NewRunner(cfg.ProviderName, cfg.ClusterPrefix, cfg.Layout, step, stateCache)
}

func NewImmutableRunnerFromConfig(cfg *config.MetaConfig, step string) *Runner {
	return NewRunner(cfg.ProviderName, cfg.ClusterPrefix, cfg.Layout, step, cache.Dummy())
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

func (r *Runner) WithHook(h InfraActionHook) *Runner {
	r.hook = h
	return r
}

func (r *Runner) WithState(stateData []byte) *Runner {
	tmpFile, err := os.CreateTemp(app.TmpDirName, r.step+deckhouseClusterStateSuffix)
	if err != nil {
		log.ErrorF("can't save terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), stateData, 0o600)
	if err != nil {
		log.ErrorF("can't write terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	r.statePath = tmpFile.Name()
	return r
}

func (r *Runner) WithVariables(variablesData []byte) *Runner {
	tmpFile, err := os.CreateTemp(app.TmpDirName, varFileName)
	if err != nil {
		log.ErrorF("can't save terraform variables for runner %s: %s\n", r.step, err)
		return r
	}

	err = os.WriteFile(tmpFile.Name(), variablesData, 0o600)
	if err != nil {
		log.ErrorF("can't write terraform variables for runner %s: %s\n", r.step, err)
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

func (r *Runner) withTerraformExecutor(t Executor) *Runner {
	r.terraformExecutor = t
	return r
}

func (r *Runner) switchTerraformIsRunning() {
	atomic.AddInt32(&r.terraformRunningCounter, 1)
}

func (r *Runner) checkTerraformIsRunning() bool {
	return (atomic.LoadInt32(&r.terraformRunningCounter) % 2) > 0
}

func (r *Runner) Init() error {
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
			log.InfoF("Cached Terraform state found:\n\t%s\n\n", r.statePath)
			if !r.allowedCachedState {
				var isConfirm bool
				switch app.UseTfCache {
				case app.UseStateCacheYes:
					isConfirm = true
				case app.UseStateCacheNo:
					isConfirm = false
				default:
					isConfirm = r.confirm().
						WithMessage("Do you want to continue with Terraform state from local cache?").
						WithYesByDefault().
						Ask()
				}

				if !isConfirm {
					return fmt.Errorf(terraformPipelineAbortedMessage)
				}
			}

			stateData, err := r.stateCache.Load(stateName)
			if err != nil {
				return err
			}

			if len(stateData) > 0 {
				err := fs.WriteContentIfNeed(r.statePath, stateData)
				if err != nil {
					err := fmt.Errorf("can't write terraform state for runner %s: %s", r.step, err)
					log.ErrorLn(err)
					return err
				}
			}
		}
	}

	// If statePath still empty, it means that something wrong with cache. Let's create file for the state in tmp directory.
	if r.statePath == "" {
		r.WithState(nil)
	}

	return log.Process("default", "terraform init ...", func() error {
		args := []string{
			"init",
			"-get-plugins=false",
			"-no-color",
			"-input=false",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			r.workingDir,
		}

		_, err := r.execTerraform(args...)
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

func (r *Runner) runBeforeActionAndWaitReady() error {
	hook := r.getHook()

	runPostAction, err := hook.BeforeAction()
	if err != nil {
		return err
	}

	if err := hook.IsReady(); err != nil {
		var resErr *multierror.Error
		resErr = multierror.Append(resErr, err)

		if runPostAction {
			err := hook.AfterAction()
			if err != nil {
				resErr = multierror.Append(resErr, err)
			}
		}

		return resErr.ErrorOrNil()
	}

	return nil
}

func (r *Runner) isSkipChanges() (skip bool, err error) {
	// first verify destructive change
	if r.changesInPlan == PlanHasDestructiveChanges && r.changeSettings.AutoDismissDestructive {
		// skip plan
		return true, nil
	}

	if r.changesInPlan == PlanHasNoChanges {
		// if plan has not changes we will run apply
		return false, nil
	}

	if !r.changeSettings.AutoApprove {
		if !r.confirm().WithMessage("Do you want to CHANGE objects state in the cloud?").Ask() {
			if r.changeSettings.SkipChangesOnDeny {
				return true, nil
			}
			return false, ErrTerraformApplyAborted
		}
	}

	err = r.runBeforeActionAndWaitReady()

	return false, err
}

func (r *Runner) Apply() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return log.Process("default", "terraform apply ...", func() error {
		skip, err := r.isSkipChanges()
		if err != nil {
			return err
		}
		if skip {
			log.InfoLn("Skip terraform apply.")
			return nil
		}

		err = r.stateSaver.Start(r)
		if err != nil {
			return err
		}
		defer r.stateSaver.Stop()

		args := []string{
			"apply",
			"-input=false",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-state=%s", r.statePath),
			fmt.Sprintf("-state-out=%s", r.statePath),
		}

		if r.planPath != "" {
			args = append(args, r.planPath)
		} else {
			args = append(args,
				fmt.Sprintf("-var-file=%s", r.variablesPath),
				r.workingDir,
			)
		}

		_, err = r.execTerraform(args...)

		var errRes *multierror.Error
		errRes = multierror.Append(errRes, err)

		// yes, do not check err from exec terraform
		// always run post action if need
		err = r.getHook().AfterAction()
		errRes = multierror.Append(errRes, err)

		return errRes.ErrorOrNil()
	})
}

func (r *Runner) Plan() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return log.Process("default", "terraform plan ...", func() error {
		tmpFile, err := os.CreateTemp(app.TmpDirName, r.step+deckhousePlanSuffix)
		if err != nil {
			return fmt.Errorf("can't create temp file for plan: %w", err)
		}

		args := []string{
			"plan",
			"-input=false",
			"-no-color",
			"-detailed-exitcode",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			fmt.Sprintf("-state=%s", r.statePath),
			fmt.Sprintf("-out=%s", tmpFile.Name()),
		}

		args = append(args, r.workingDir)

		exitCode, err := r.execTerraform(args...)
		if exitCode == terraformHasChangesExitCode {
			r.changesInPlan = PlanHasChanges
			destructiveChanges, err := r.getPlanDestructiveChanges(tmpFile.Name())
			if err != nil {
				return err
			}
			if destructiveChanges != nil {
				r.changesInPlan = PlanHasDestructiveChanges
				r.planDestructiveChanges = destructiveChanges
			}
		} else if err != nil {
			return err
		}

		r.planPath = tmpFile.Name()

		return nil
	})
}

func (r *Runner) GetTerraformOutput(output string) ([]byte, error) {
	if r.stopped {
		return nil, ErrRunnerStopped
	}

	if r.statePath == "" {
		return nil, fmt.Errorf("no state found, try to run terraform apply first")
	}
	args := []string{
		"output",
		"-no-color",
		"-json",
		fmt.Sprintf("-state=%s", r.statePath),
	}
	args = append(args, output)

	result, err := r.terraformExecutor.Output(args...)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("can't get terraform output for %q\n%v", output, err)
	}

	return result, nil
}

func (r *Runner) Destroy() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	if r.statePath == "" {
		return fmt.Errorf("no state found, try to run terraform apply first")
	}

	if r.changeSettings.AutoDismissDestructive {
		log.InfoLn("terraform destroy skipped")
		return nil
	}

	if !r.changeSettings.AutoApprove {
		if !r.confirm().WithMessage("Do you want to DELETE objects from the cloud?").Ask() {
			return fmt.Errorf("terraform destroy aborted")
		}
	}

	err := r.runBeforeActionAndWaitReady()
	if err != nil {
		return err
	}

	return log.Process("default", "terraform destroy ...", func() error {
		err := r.stateSaver.Start(r)
		if err != nil {
			return err
		}
		defer r.stateSaver.Stop()

		args := []string{
			"destroy",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			fmt.Sprintf("-state=%s", r.statePath),
		}
		args = append(args, r.workingDir)

		_, err = r.execTerraform(args...)

		var errRes *multierror.Error
		errRes = multierror.Append(errRes, err)

		// yes, do not check err from exec terraform
		// always run post action if need
		err = r.getHook().AfterAction()
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
		log.ErrorLn(err)
		return 0
	}

	var st struct {
		Resources []json.RawMessage `json:"resources"`
	}
	err = json.Unmarshal(data, &st)
	if err != nil {
		log.ErrorLn(err)
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

func (r *Runner) GetPlanDestructiveChanges() *PlanDestructiveChanges {
	return r.planDestructiveChanges
}

func (r *Runner) GetPlanPath() string {
	return r.planPath
}

func (r *Runner) GetTerraformExecutor() Executor {
	return r.terraformExecutor
}

// Stop interrupts the current runner command and sets
// a flag to prevent executions of next runner commands.
func (r *Runner) Stop() {
	if r.checkTerraformIsRunning() && !r.stopped {
		log.DebugF("Runner Stop is called for %s.\n", r.name)
		r.terraformExecutor.Stop()
	}
	r.stopped = true
	// Wait until the running terraform command stops.
	for r.checkTerraformIsRunning() {
		time.Sleep(50 * time.Millisecond)
	}
	// Wait until the StateSaver saves the Secret for Apply and Destroy commands.
	if r.stateSaver.IsStarted() {
		<-r.stateSaver.DoneCh()
	}
}

func (r *Runner) execTerraform(args ...string) (int, error) {
	if r.checkTerraformIsRunning() {
		return 0, fmt.Errorf("Terraform have been already executed.")
	}

	r.switchTerraformIsRunning()
	defer r.switchTerraformIsRunning()

	exitCode, err := r.terraformExecutor.Exec(args...)
	log.InfoF("Terraform runner %q process exited.\n", r.step)

	return exitCode, err
}

type TerraformPlan map[string]any

type PlanDestructiveChanges struct {
	ResourcesDeleted   []ValueChange `json:"resources_deleted,omitempty"`
	ResourcesRecreated []ValueChange `json:"resourced_recreated,omitempty"`
}

type ValueChange struct {
	CurrentValue interface{} `json:"current_value,omitempty"`
	NextValue    interface{} `json:"next_value,omitempty"`
}

func (r *Runner) getPlanDestructiveChanges(planFile string) (*PlanDestructiveChanges, error) {
	args := []string{
		"show",
		"-json",
		planFile,
	}

	result, err := r.terraformExecutor.Output(args...)
	if err != nil {
		var ee *exec.ExitError
		if ok := errors.As(err, &ee); ok {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("can't get terraform plan for %q\n%v", planFile, err)
	}

	os.WriteFile(fmt.Sprintf("/%x.json", md5.Sum([]byte(planFile))), result, os.ModePerm)

	var changes struct {
		ResourcesChanges []struct {
			Change struct {
				Actions []string               `json:"actions"`
				Before  map[string]interface{} `json:"before,omitempty"`
				After   map[string]interface{} `json:"after,omitempty"`
			} `json:"change"`
		} `json:"resource_changes"`
	}

	err = json.Unmarshal(result, &changes)
	if err != nil {
		return nil, err
	}

	var destructiveChanges *PlanDestructiveChanges
	getOrCreateDestructiveChanges := func() *PlanDestructiveChanges {
		if destructiveChanges == nil {
			destructiveChanges = &PlanDestructiveChanges{}
		}
		return destructiveChanges
	}

	for _, resource := range changes.ResourcesChanges {
		if hasAction(resource.Change.Actions, "delete") {
			if hasAction(resource.Change.Actions, "create") {
				// recreate
				getOrCreateDestructiveChanges().ResourcesRecreated = append(getOrCreateDestructiveChanges().ResourcesRecreated, ValueChange{
					CurrentValue: resource.Change.Before,
					NextValue:    resource.Change.After,
				})
			} else {
				getOrCreateDestructiveChanges().ResourcesDeleted = append(getOrCreateDestructiveChanges().ResourcesDeleted, ValueChange{
					CurrentValue: resource.Change.Before,
				})
			}
		}
	}

	return destructiveChanges, nil
}

func hasAction(actions []string, findAction string) bool {
	for _, action := range actions {
		if action == findAction {
			return true
		}
	}
	return false
}

func buildTerraformPath(provider, layout, step string) string {
	return filepath.Join(cloudProvidersDir, provider, "layouts", layout, step)
}
