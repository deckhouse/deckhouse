package terraform

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
)

const (
	deckhouseClusterStatePrefix = ".deckhouse-candi.tfstate"
	cloudProvidersDir           = "/deckhouse/candi/cloud-providers/"
	varFileName                 = "cluster-config.auto.tfvars.json"
)

type Interface interface {
	Init(bool) ([]byte, error)
	Apply() ([]byte, error)
	GetTerraformOutput(string) ([]byte, error)
	getState() ([]byte, error)
}

type Runner struct {
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
	return &Runner{WorkingDir: workingDir, MetaConfig: metaConfig}
}

func (r *Runner) Init(bootstrap bool) ([]byte, error) {
	logboek.LogInfoF("Init terraform ... ")

	clusterConfigJSON, err := r.MetaConfig.MarshalConfig(bootstrap)
	if err != nil {
		return nil, fmt.Errorf("terraform prepare cluster config error: %v", err)
	}

	varFilePath := filepath.Join(r.WorkingDir, varFileName)
	if err = ioutil.WriteFile(varFilePath, clusterConfigJSON, 0755); err != nil {
		return nil, fmt.Errorf("terraform saving cluster config error: %v", err)
	}

	output, err := exec.Command("terraform",
		"init",
		"-get-plugins=false",
		"-no-color",
		"-input=false",
		fmt.Sprintf("-var-file=%s", varFilePath),
		r.WorkingDir,
	).CombinedOutput() // #nosec

	if err == nil {
		logboek.LogInfoLn("OK!")
	} else {
		logboek.LogInfoLn("ERROR!")
	}
	return output, err
}

func (r *Runner) Apply() ([]byte, error) {
	logboek.LogInfoF("Apply terraform ... ")
	state := filepath.Join(r.WorkingDir, deckhouseClusterStatePrefix)
	args := []string{
		"apply",
		"-auto-approve",
		"-input=false",
		"-no-color",
		fmt.Sprintf("-var-file=%s", filepath.Join(r.WorkingDir, varFileName)),
		fmt.Sprintf("-state=%s", state),
		fmt.Sprintf("-state-out=%s", state),
		r.WorkingDir,
	}
	data, err := exec.Command("terraform", args...).CombinedOutput() // #nosec
	if err == nil {
		r.State = state
		logboek.LogInfoLn("OK!")
	} else {
		logboek.LogInfoLn("ERROR!")
	}
	return data, err
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
		r.State = filepath.Join(r.WorkingDir, deckhouseClusterStatePrefix)
	}
	args := []string{
		"destroy",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-var-file=%s", filepath.Join(r.WorkingDir, varFileName)),
		fmt.Sprintf("-state=%s", r.State),
		r.WorkingDir,
	}
	return exec.Command("terraform", args...).CombinedOutput()
}

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.State)
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

func (r *FakeRunner) getState() ([]byte, error) {
	return []byte(r.State), nil
}
