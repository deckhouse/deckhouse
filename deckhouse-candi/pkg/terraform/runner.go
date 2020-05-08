package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
)

const (
	deckhouseClusterStateSuffix = "-deckhouse-candi.tfstate"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.tfvars.json"
)

type Interface interface {
	Init(bool) ([]byte, error)
	Apply() ([]byte, error)
	GetTerraformOutput(string) ([]byte, error)
	Destroy(bool) ([]byte, error)
	getState() ([]byte, error)
}

type Runner struct {
	step       string
	stateDir   string
	WorkingDir string
	State      string
	MetaConfig *config.MetaConfig
}

var (
	_ Interface = &Runner{}
	_ Interface = &FakeRunner{}
)

func NewRunner(step string, metaConfig *config.MetaConfig) *Runner {
	workingDir := buildTerraformPath(metaConfig.ProviderName, metaConfig.Layout, step)
	return &Runner{WorkingDir: workingDir, stateDir: workingDir, step: step, MetaConfig: metaConfig}
}

func (r *Runner) WithStateDir(dir string) {
	if dir != "" {
		r.stateDir = dir
	}
}

func (r *Runner) Init(bootstrap bool) ([]byte, error) {
	err := logboek.LogProcess("Terraform Init", log.TerraformOptions(), func() error {
		clusterConfigJSON, err := r.MetaConfig.MarshalConfig(bootstrap)
		if err != nil {
			return fmt.Errorf("terraform prepare cluster config error: %v", err)
		}

		varFilePath := filepath.Join(r.stateDir, varFileName)
		if err = ioutil.WriteFile(varFilePath, clusterConfigJSON, 0755); err != nil {
			return fmt.Errorf("terraform saving cluster config error: %v", err)
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
	return []byte(""), err
}

func (r *Runner) Apply() ([]byte, error) {
	err := logboek.LogProcess("Terraform Apply", log.TerraformOptions(), func() error {
		state := filepath.Join(r.stateDir, r.step+deckhouseClusterStateSuffix)
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

func (r *Runner) Destroy(detectState bool) ([]byte, error) {
	if r.State == "" {
		if !detectState {
			return nil, fmt.Errorf("no state found, try to run terraform apply first")
		}
		r.State = filepath.Join(r.stateDir, r.step+deckhouseClusterStateSuffix)
	}

	err := logboek.LogProcess("Terraform Destroy", log.TerraformOptions(), func() error {
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
	return []byte(""), err
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

func (r *FakeRunner) Init(_ bool) ([]byte, error) {
	return r.InitResult.Data, r.InitResult.Error
}

func (r *FakeRunner) Apply() ([]byte, error) {
	return r.ApplyResult.Data, r.ApplyResult.Error
}

func (r *FakeRunner) GetTerraformOutput(output string) ([]byte, error) {
	result := r.OutputResults[output]
	return result.Data, result.Error
}

func (r *FakeRunner) Destroy(_ bool) ([]byte, error) { return nil, nil }

func (r *FakeRunner) getState() ([]byte, error) {
	return []byte(r.State), nil
}
