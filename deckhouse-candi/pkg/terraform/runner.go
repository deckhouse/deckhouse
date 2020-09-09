package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/util/cache"
	"flant/deckhouse-candi/pkg/util/retry"
)

const (
	deckhouseClusterStateSuffix = "-deckhouse-candi.*.tfstate"
	deckhousePlanSuffix         = "-deckhouse-candi.*.tfplan"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.*.tfvars.json"

	terraformHasChangesExitCode = 2
)

type Interface interface {
	Init() error
	Apply() error
	Destroy() error
	Close()
	GetTerraformOutput(string) ([]byte, error)
	getState() ([]byte, error)
}

type Runner struct {
	name       string
	prefix     string
	step       string
	workingDir string

	statePath     string
	planPath      string
	variablesPath string

	autoApprove   bool
	changesInPlan bool

	stateCache cache.Cache
}

var (
	_ Interface = &Runner{}
	_ Interface = &FakeRunner{}
)

func NewRunner(provider, prefix, layout, step string) *Runner {
	return &Runner{
		prefix:     prefix,
		step:       step,
		name:       step,
		workingDir: buildTerraformPath(provider, layout, step),
		stateCache: cache.Global(),
	}
}

func NewRunnerFromConfig(cfg *config.MetaConfig, step string) *Runner {
	return NewRunner(cfg.ProviderName, cfg.ClusterPrefix, cfg.Layout, step)
}

func (r *Runner) WithName(name string) *Runner {
	r.name = name
	return r
}

func (r *Runner) WithState(stateData []byte) *Runner {
	tmpFile, err := ioutil.TempFile(app.TmpDirName, r.step+deckhouseClusterStateSuffix)
	if err != nil {
		log.ErrorF("can't save terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	err = ioutil.WriteFile(tmpFile.Name(), stateData, 0755)
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

	err = ioutil.WriteFile(tmpFile.Name(), variablesData, 0755)
	if err != nil {
		log.ErrorF("can't write terraform variables for runner %s: %s\n", r.step, err)
		return r
	}

	r.variablesPath = tmpFile.Name()
	return r
}

func (r *Runner) WithAutoApprove(autoApprove bool) *Runner {
	r.autoApprove = autoApprove
	return r
}

func (r *Runner) Init() error {
	if r.statePath == "" && r.stateCache.InCache(r.name) {
		r.statePath = r.stateCache.ObjectPath(r.name)

		log.InfoF("Cached Terraform state found: %s\n\n", r.statePath)
		if !retry.AskForConfirmation("Do you want to continue with Terraform state from local cash") {
			return fmt.Errorf("Terraform pipeline aborted.\nIf you want to drop the cache and continue, please run deckhouse-candi with '--yes-i-want-to-drop-cache' flag.")
		}
	} else if r.statePath == "" {
		tmpFile, err := ioutil.TempFile(app.TmpDirName, r.step+deckhouseClusterStateSuffix)
		if err != nil {
			return fmt.Errorf("can't save terraform variables for runner %s: %s", r.step, err)
		}
		r.statePath = tmpFile.Name()
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

		_, err := execTerraform(args...)
		return err
	})
}

func (r *Runner) Apply() error {
	return log.Process("default", "terraform apply ...", func() error {
		if !r.autoApprove && r.changesInPlan {
			if !retry.AskForConfirmation("Do you want to CHANGE objects state in the cloud") {
				return fmt.Errorf("terraform apply aborted")
			}
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

		_, err := execTerraform(args...)
		if err != nil {
			return err
		}

		r.stateCache.SaveByPath(r.name, r.statePath)
		return nil
	})
}

func (r *Runner) Plan() error {
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

		exitCode, err := execTerraform(args...)
		if exitCode == terraformHasChangesExitCode {
			r.changesInPlan = true
		} else if err != nil {
			return err
		}

		r.planPath = tmpFile.Name()
		return nil
	})
}

func (r *Runner) GetTerraformOutput(output string) ([]byte, error) {
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
	result, err := exec.Command("terraform", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("can't get terraform output for %q\n%v", output, err)
	}

	r.stateCache.AddToClean(r.name)
	return result, nil
}

func (r *Runner) Destroy() error {
	if r.statePath == "" {
		return fmt.Errorf("no state found, try to run terraform apply first")
	}

	r.stateCache.SaveByPath(r.name, r.statePath)
	if !r.autoApprove {
		if !retry.AskForConfirmation("Do you want to DELETE objects from the cloud") {
			return fmt.Errorf("terraform destroy aborted")
		}
	}

	return log.Process("default", "terraform destroy ...", func() error {
		args := []string{
			"destroy",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			fmt.Sprintf("-state=%s", r.statePath),
		}
		args = append(args, r.workingDir)

		if _, err := execTerraform(args...); err != nil {
			return err
		}

		r.stateCache.Delete(r.name)
		return nil
	})
}

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.statePath)
}

func (r *Runner) Close() {
	_ = os.Remove(r.variablesPath)
}

func execTerraform(args ...string) (int, error) {
	cmd := exec.Command("terraform", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("can't open stdout pipe: %v", err)
	}

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	cmd.Stdin = os.Stdin

	cmd.Env = append(cmd.Env, "TF_IN_AUTOMATION=yes")

	err = cmd.Start()
	if err != nil {
		log.ErrorF("%s\n%v\n", errBuf.String(), err)
		return cmd.ProcessState.ExitCode(), err
	}

	r := bufio.NewScanner(stdout)
	for r.Scan() {
		log.InfoLn(r.Text())
	}

	err = cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode() // 2 = exit code, if terraform plan has diff
	if err != nil && exitCode != terraformHasChangesExitCode {
		log.ErrorLn(err)
		err = fmt.Errorf(errBuf.String())
	}
	return exitCode, err
}

func buildTerraformPath(provider, layout, step string) string {
	return filepath.Join(cloudProvidersDir, provider, "layouts", layout, step)
}

type fakeResult struct {
	Data  []byte
	Error error
}

type FakeRunner struct {
	State         string
	InitResult    fakeResult
	ApplyResult   fakeResult
	OutputResults map[string]fakeResult
}

func (r *FakeRunner) Init() error {
	return r.InitResult.Error
}

func (r *FakeRunner) Apply() error {
	return r.ApplyResult.Error
}

func (r *FakeRunner) GetTerraformOutput(output string) ([]byte, error) {
	result := r.OutputResults[output]
	return result.Data, result.Error
}

func (r *FakeRunner) Destroy() error { return nil }

func (r *FakeRunner) Close() {}

func (r *FakeRunner) getState() ([]byte, error) {
	return []byte(r.State), nil
}
