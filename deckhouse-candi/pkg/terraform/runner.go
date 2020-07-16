package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/log"
)

const (
	deckhouseClusterStateSuffix = "-deckhouse-candi.tfstate"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.tfvars.json"
)

type Interface interface {
	Init() error
	Apply() ([]byte, error)
	GetTerraformOutput(string) ([]byte, error)
	Destroy(bool) error
	getState() ([]byte, error)
}

type Runner struct {
	step               string
	stateDir           string
	stateSuffix        string
	WorkingDir         string
	State              string
	TerraformVariables []byte
}

var (
	_ Interface = &Runner{}
	_ Interface = &FakeRunner{}
)

func NewRunner(provider, layout, step string, terraformVariables []byte) *Runner {
	workingDir := buildTerraformPath(provider, layout, step)
	return &Runner{WorkingDir: workingDir, stateDir: workingDir, step: step, TerraformVariables: terraformVariables}
}

func (r *Runner) WithStateDir(dir string) *Runner {
	if dir != "" {
		r.stateDir = dir
	}
	return r
}

func (r *Runner) WithStateSuffix(suffix string) *Runner {
	if suffix != "" {
		r.stateSuffix = suffix
	}
	return r
}

func (r *Runner) Init() error {
	return logboek.LogProcess("Terraform Init", log.TerraformOptions(), func() error {
		varFilePath := filepath.Join(r.stateDir, varFileName)
		if err := ioutil.WriteFile(varFilePath, r.TerraformVariables, 0755); err != nil {
			return fmt.Errorf("terraform saving cluster config error: %w", err)
		}

		args := []string{
			"init",
			"-get-plugins=false",
			"-no-color",
			"-input=false",
			fmt.Sprintf("-var-file=%s", varFilePath),
			r.WorkingDir,
		}

		return execTerraform(args...)
	})
}

func (r *Runner) Apply() ([]byte, error) {
	err := logboek.LogProcess("Terraform Apply", log.TerraformOptions(), func() error {
		state := filepath.Join(r.stateDir, r.step+r.stateSuffix+deckhouseClusterStateSuffix)
		args := []string{
			"apply",
			"-auto-approve",
			"-input=false",
			"-no-color",
			fmt.Sprintf("-var-file=%s", filepath.Join(r.stateDir, varFileName)),
			fmt.Sprintf("-state=%s", state),
			fmt.Sprintf("-state-out=%s", state),
			r.WorkingDir,
		}

		err := execTerraform(args...)
		if err == nil {
			r.State = state
		}
		return err
	})
	return []byte(""), err
}

func (r *Runner) GetTerraformOutput(output string) ([]byte, error) {
	if r.State == "" {
		return nil, fmt.Errorf("no state found, try to run terraform apply first")
	}
	args := []string{
		"output",
		"-no-color",
		"-json",
		fmt.Sprintf("-state=%s", r.State),
	}
	args = append(args, output)
	return exec.Command("terraform", args...).CombinedOutput()
}

func (r *Runner) Destroy(detectState bool) error {
	if r.State == "" {
		if !detectState {
			return fmt.Errorf("no state found, try to run terraform apply first")
		}
		r.State = filepath.Join(r.stateDir, r.step+r.stateSuffix+deckhouseClusterStateSuffix)
	}

	return logboek.LogProcess("Terraform Destroy", log.TerraformOptions(), func() error {
		args := []string{
			"destroy",
			"-no-color",
			"-auto-approve",
			fmt.Sprintf("-var-file=%s", filepath.Join(r.stateDir, varFileName)),
			fmt.Sprintf("-state=%s", r.State),
			r.WorkingDir,
		}

		return execTerraform(args...)
	})
}

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.State)
}

func execTerraform(args ...string) error {
	cmd := exec.Command("terraform", args...)
	stdout, _ := cmd.StdoutPipe()

	var errbuf bytes.Buffer
	cmd.Stderr = &errbuf

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("run terraform: %v", err)
	}

	r := bufio.NewScanner(stdout)
	for r.Scan() {
		logboek.LogInfoLn(r.Text())
	}

	err = cmd.Wait()
	if err != nil {
		logboek.LogWarnF(errbuf.String() + "\n")
		return fmt.Errorf("wait terraform: %v", err)
	}
	return nil
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

func (r *FakeRunner) Apply() ([]byte, error) {
	return r.ApplyResult.Data, r.ApplyResult.Error
}

func (r *FakeRunner) GetTerraformOutput(output string) ([]byte, error) {
	result := r.OutputResults[output]
	return result.Data, result.Error
}

func (r *FakeRunner) Destroy(_ bool) error { return nil }

func (r *FakeRunner) getState() ([]byte, error) {
	return []byte(r.State), nil
}
