package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
)

const (
	deckhouseClusterStateSuffix = "-deckhouse-candi.*.tfstate"
	deckhousePlanSuffix         = "-deckhouse-candi.*.tfplan"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.*.tfvars.json"

	terraformHasChangesExitCode = 2
)

var deckhouseCandiTemporaryDirName = filepath.Join(os.TempDir(), "deckhouse-candi")

func init() {
	_ = os.Mkdir(deckhouseCandiTemporaryDirName, 0755)
}

type Interface interface {
	Init() error
	Apply() error
	Destroy() error
	Close()
	GetTerraformOutput(string) ([]byte, error)
	getState() ([]byte, error)
}

type Runner struct {
	step       string
	workingDir string

	statePath     string
	planPath      string
	variablesPath string

	autoApprove   bool
	changesInPlan bool
}

var (
	_ Interface = &Runner{}
	_ Interface = &FakeRunner{}
)

func NewRunner(provider, layout, step string) *Runner {
	workingDir := buildTerraformPath(provider, layout, step)
	return &Runner{workingDir: workingDir, step: step}
}

func NewRunnerFromConfig(metaConfig *config.MetaConfig, step string) *Runner {
	return NewRunner(metaConfig.ProviderName, metaConfig.Layout, step)
}

func (r *Runner) WithStatePath(state string) *Runner {
	if state != "" {
		r.statePath = state
	} else {
		tmpFile, err := ioutil.TempFile(deckhouseCandiTemporaryDirName, r.step+deckhouseClusterStateSuffix)
		if err != nil {
			logboek.LogWarnF("can't save terraform variables for runner %s: %s\n", r.step, err)
			return r
		}
		r.statePath = tmpFile.Name()
	}
	return r
}

func (r *Runner) WithState(stateData []byte) *Runner {
	tmpFile, err := ioutil.TempFile(deckhouseCandiTemporaryDirName, r.step+deckhouseClusterStateSuffix)
	if err != nil {
		logboek.LogWarnF("can't save terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	err = ioutil.WriteFile(tmpFile.Name(), stateData, 0755)
	if err != nil {
		logboek.LogWarnF("can't write terraform state for runner %s: %s\n", r.step, err)
		return r
	}

	r.statePath = tmpFile.Name()
	return r
}

func (r *Runner) WithVariablesPath(variables string) *Runner {
	if variables != "" {
		r.variablesPath = variables
	} else {
		tmpFile, err := ioutil.TempFile(deckhouseCandiTemporaryDirName, varFileName)
		if err != nil {
			logboek.LogWarnF("can't save terraform variables for runner %s: %s\n", r.step, err)
			return r
		}
		r.statePath = tmpFile.Name()
	}
	return r
}

func (r *Runner) WithVariables(variablesData []byte) *Runner {
	tmpFile, err := ioutil.TempFile(deckhouseCandiTemporaryDirName, varFileName)
	if err != nil {
		logboek.LogWarnF("can't save terraform variables for runner %s: %s\n", r.step, err)
		return r
	}

	err = ioutil.WriteFile(tmpFile.Name(), variablesData, 0755)
	if err != nil {
		logboek.LogWarnF("can't write terraform variables for runner %s: %s\n", r.step, err)
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
	return log.BoldProcess("terraform init ...", func() error {
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
	return log.BoldProcess("terraform apply ...", func() error {
		if !r.autoApprove && r.changesInPlan {
			if !askForConfirmation("Do you want to CHANGE objects state in the cloud?") {
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
		return err
	})
}

func (r *Runner) Plan() error {
	return log.BoldProcess("terraform plan ...", func() error {
		tmpFile, err := ioutil.TempFile(deckhouseCandiTemporaryDirName, r.step+deckhousePlanSuffix)
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
	result, err := exec.Command("terraform", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("can't get terraform output for %q\n%s\n%w", output, string(result), err)
	}
	return result, nil
}

func (r *Runner) Destroy() error {
	if r.statePath == "" {
		return fmt.Errorf("no state found, try to run terraform apply first")
	}

	if !r.autoApprove {
		if !askForConfirmation("Do you want to DELETE objects from the cloud?") {
			return fmt.Errorf("terraform destroy aborted")
		}
	}

	return log.BoldProcess("terraform destroy ...", func() error {
		args := []string{
			"destroy",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-var-file=%s", r.variablesPath),
			fmt.Sprintf("-state=%s", r.statePath),
		}
		args = append(args, r.workingDir)

		_, err := execTerraform(args...)
		return err
	})
}

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.statePath)
}

func (r *Runner) Close() {
	_ = os.Remove(r.statePath)
	_ = os.Remove(r.variablesPath)
}

func execTerraform(args ...string) (int, error) {
	cmd := exec.Command("terraform", args...)
	stdout, _ := cmd.StdoutPipe()

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	cmd.Stdin = os.Stdin

	cmd.Env = append(cmd.Env, "TF_IN_AUTOMATION=yes")

	err := cmd.Start()
	if err != nil {
		logboek.LogWarnF("%s\n%v\n", errBuf.String(), err)
		return cmd.ProcessState.ExitCode(), err
	}

	r := bufio.NewScanner(stdout)
	for r.Scan() {
		logboek.LogInfoLn(r.Text())
	}

	err = cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode() // 2 = exit code, if terraform plan has diff
	if err != nil && exitCode != terraformHasChangesExitCode {
		logboek.LogErrorLn(err)
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

func askForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("~~~~~~~~~~")
		fmt.Printf("%s [y/n]: ", s)

		line, _, err := reader.ReadLine()
		if err != nil {
			logboek.LogWarnF("can't read from stdin: %v\n", err)
			return false
		}

		response := strings.ToLower(strings.TrimSpace(string(line)))

		if response == "y" || response == "yes" {
			fmt.Println("~~~~~~~~~~")
			return true
		} else if response == "n" || response == "no" {
			fmt.Println("~~~~~~~~~~")
			return false
		}
	}
}
