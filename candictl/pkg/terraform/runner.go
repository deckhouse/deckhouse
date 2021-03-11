package terraform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/config"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/input"
)

const (
	deckhouseClusterStateSuffix = "-candictl.*.tfstate"
	deckhousePlanSuffix         = "-candictl.*.tfplan"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.*.tfvars.json"

	terraformHasChangesExitCode = 2

	terraformPipelineAbortedMessage = `
Terraform pipeline aborted.
If you want to drop the cache and continue, please run candictl with "--yes-i-want-to-drop-cache" flag.
`
)

const (
	PlanHasNoChanges = iota
	PlanHasChanges
	PlanHasDestructiveChanges
)

var (
	ErrRunnerStopped         = errors.New("Terraform runner was stopped.")
	ErrTerraformApplyAborted = errors.New("terraform apply aborted")
)

type Runner struct {
	name       string
	prefix     string
	step       string
	workingDir string

	statePath     string
	planPath      string
	variablesPath string

	autoApprove        bool
	allowedCachedState bool
	skipChangesOnDeny  bool
	changesInPlan      int

	stateCache cache.Cache

	cmd     *exec.Cmd
	confirm func() *input.Confirmation
	stopped bool
}

func NewRunner(provider, prefix, layout, step string) *Runner {
	return &Runner{
		prefix:     prefix,
		step:       step,
		name:       step,
		workingDir: buildTerraformPath(provider, layout, step),
		confirm:    input.NewConfirmation,
		stateCache: cache.Global(),
	}
}

func NewRunnerFromConfig(cfg *config.MetaConfig, step string) *Runner {
	return NewRunner(cfg.ProviderName, cfg.ClusterPrefix, cfg.Layout, step)
}

func (r *Runner) WithCache(cache cache.Cache) *Runner {
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

func (r *Runner) WithState(stateData []byte) *Runner {
	tmpFile, err := ioutil.TempFile(app.TmpDirName, r.step+deckhouseClusterStateSuffix)
	if err != nil {
		log.ErrorF("can't save terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	err = ioutil.WriteFile(tmpFile.Name(), stateData, 0600)
	if err != nil {
		log.ErrorF("can't write terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	r.statePath = tmpFile.Name()
	return r
}

func (r *Runner) WithVariables(variablesData []byte) *Runner {
	tmpFile, err := ioutil.TempFile(app.TmpDirName, varFileName)
	if err != nil {
		log.ErrorF("can't save terraform variables for runner %s: %s\n", r.step, err)
		return r
	}

	err = ioutil.WriteFile(tmpFile.Name(), variablesData, 0600)
	if err != nil {
		log.ErrorF("can't write terraform variables for runner %s: %s\n", r.step, err)
		return r
	}

	r.variablesPath = tmpFile.Name()
	return r
}

func (r *Runner) WithAutoApprove(flag bool) *Runner {
	r.autoApprove = flag
	return r
}

func (r *Runner) WithAllowedCachedState(flag bool) *Runner {
	r.allowedCachedState = flag
	return r
}

func (r *Runner) WithSkipChangesOnDeny(flag bool) *Runner {
	r.skipChangesOnDeny = flag
	return r
}

func (r *Runner) Init() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	if r.statePath == "" {
		// Save state directly in the cache to prevent state loss
		stateName := fmt.Sprintf("%s.tfstate", r.name)
		r.statePath = r.stateCache.GetPath(stateName)

		if r.stateCache.InCache(stateName) && !r.allowedCachedState {
			log.InfoF("Cached Terraform state found:\n\t%s\n\n", r.statePath)
			if !r.confirm().
				WithMessage("Do you want to continue with Terraform state from local cache?").
				WithYesByDefault().
				Ask() {
				return fmt.Errorf(terraformPipelineAbortedMessage)
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

func (r *Runner) handleChanges() (bool, error) {
	if r.autoApprove || r.changesInPlan == PlanHasNoChanges {
		return false, nil
	}

	if !r.confirm().WithMessage("Do you want to CHANGE objects state in the cloud?").Ask() {
		if r.skipChangesOnDeny {
			return true, nil
		}
		return false, ErrTerraformApplyAborted
	}

	return false, nil
}

func (r *Runner) Apply() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return log.Process("default", "terraform apply ...", func() error {
		skip, err := r.handleChanges()
		if err != nil {
			return err
		}
		if skip {
			log.InfoLn("Skip terraform apply.")
			return nil
		}

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
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *Runner) Plan() error {
	if r.stopped {
		return ErrRunnerStopped
	}

	return log.Process("default", "terraform plan ...", func() error {
		tmpFile, err := ioutil.TempFile(app.TmpDirName, r.step+deckhousePlanSuffix)
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
			hasDestructiveChanges, err := checkPlanDestructiveChanges(tmpFile.Name())
			if err != nil {
				return err
			}
			if hasDestructiveChanges {
				r.changesInPlan = PlanHasDestructiveChanges
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

	result, err := terraformCmd(args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("can't get terraform output for %q\n%v", output, err)
	}

	return result, nil
}

func (r *Runner) Destroy() error {
	if r.statePath == "" {
		return fmt.Errorf("no state found, try to run terraform apply first")
	}

	if !r.autoApprove {
		if !r.confirm().WithMessage("Do you want to DELETE objects from the cloud?").Ask() {
			return fmt.Errorf("terraform destroy aborted")
		}
	}

	r.stopped = true
	return log.Process("default", "terraform destroy ...", func() error {
		args := []string{
			"destroy",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			fmt.Sprintf("-state=%s", r.statePath),
		}
		args = append(args, r.workingDir)

		if _, err := r.execTerraform(args...); err != nil {
			return err
		}

		return nil
	})
}

func (r *Runner) ResourcesQuantityInState() int {
	if r.statePath == "" {
		return 0
	}

	data, err := ioutil.ReadFile(r.statePath)
	if err != nil {
		log.ErrorLn(err)
		return 0
	}

	var state struct {
		Resources []json.RawMessage `json:"resources"`
	}
	err = json.Unmarshal(data, &state)
	if err != nil {
		log.ErrorLn(err)
		return 0
	}

	return len(state.Resources)
}

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.statePath)
}

func (r *Runner) Stop() {
	r.stopped = true
	for r.cmd != nil {
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *Runner) execTerraform(args ...string) (int, error) {
	r.cmd = terraformCmd(args...)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %v", err)
	}

	log.DebugLn(r.cmd.String())
	err = r.cmd.Start()
	if err != nil {
		log.ErrorLn(err)
		return r.cmd.ProcessState.ExitCode(), err
	}

	var errBuf bytes.Buffer
	waitCh := make(chan error)
	go func() {
		e := bufio.NewScanner(stderr)
		for e.Scan() {
			if app.IsDebug {
				log.DebugLn(e.Text())
			} else {
				errBuf.WriteString(e.Text() + "\n")
			}
		}

		waitCh <- r.cmd.Wait()
	}()

	s := bufio.NewScanner(stdout)
	for s.Scan() {
		log.InfoLn(s.Text())
	}

	err = <-waitCh
	log.InfoF("Terraform runner %q process exited.\n", r.step)

	exitCode := r.cmd.ProcessState.ExitCode() // 2 = exit code, if terraform plan has diff
	if err != nil && exitCode != terraformHasChangesExitCode {
		log.ErrorLn(err)
		err = fmt.Errorf(errBuf.String())
		if app.IsDebug {
			err = fmt.Errorf("terraform has failed in DEBUG mode, search in the output above for an error")
		}
	}
	r.cmd = nil

	if exitCode == 0 {
		err = nil
	}
	return exitCode, err
}

func buildTerraformPath(provider, layout, step string) string {
	return filepath.Join(cloudProvidersDir, provider, "layouts", layout, step)
}

func terraformCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("terraform", args...)
	cmd.Env = append(
		cmd.Env,
		"TF_IN_AUTOMATION=yes", "TF_DATA_DIR="+filepath.Join(app.TmpDirName, "tf_candictl"),
	)
	if app.IsDebug {
		// Debug mode is deprecated, however trace produces more useless information
		cmd.Env = append(cmd.Env, "TF_LOG=DEBUG")
	}
	return cmd
}

func checkPlanDestructiveChanges(planFile string) (bool, error) {
	args := []string{
		"show",
		"-json",
		planFile,
	}

	result, err := terraformCmd(args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return false, fmt.Errorf("can't get terraform plan for %q\n%v", planFile, err)
	}

	var changes struct {
		ResourcesChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}

	err = json.Unmarshal(result, &changes)
	if err != nil {
		return false, err
	}

	hasDestructiveChanges := func() bool {
		for _, resource := range changes.ResourcesChanges {
			for _, action := range resource.Change.Actions {
				if action == "delete" {
					return true
				}
			}
		}
		return false
	}

	return hasDestructiveChanges(), nil
}
