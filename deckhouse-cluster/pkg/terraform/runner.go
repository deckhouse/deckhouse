package terraform

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	// "github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"

	"flant/deckhouse-cluster/pkg/config"
)

const (
	deckhouseClusterStatePrefix = ".deckhouse-cluster.tfstate"
	cloudProviderPrefix         = "030-cloud-provider-"
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
	log.Infof("Start init process")

	clusterConfigJSON, err := r.MetaConfig.MarshalConfig(bootstrap)
	if err != nil {
		return nil, fmt.Errorf("terraform prepare cluster config error: %v", err)
	}
	err = ioutil.WriteFile(
		filepath.Join(r.WorkingDir, "cluster-config.auto.tfvars.json"),
		clusterConfigJSON,
		0755,
	)
	if err != nil {
		return nil, fmt.Errorf("terraform saving cluster config error: %v", err)
	}
	log.Infof("cluster-config.auto.tfvars.json saved")

	return exec.Command("terraform",
		"init",
		"-get-plugins=false",
		"-no-color",
		"-input=false",
		r.WorkingDir,
	).CombinedOutput() // #nosec
}

func (r *Runner) Apply() ([]byte, error) {
	state := filepath.Join(r.WorkingDir, deckhouseClusterStatePrefix)
	args := []string{
		"apply",
		"-auto-approve",
		"-input=false",
		"-no-color",
		fmt.Sprintf("-state-out=%s", state),
		r.WorkingDir,
	}
	data, err := exec.Command("terraform", args...).CombinedOutput() // #nosec
	if err == nil {
		r.State = state
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

func (r *Runner) getState() ([]byte, error) {
	return ioutil.ReadFile(r.State)
}

func buildTerraformPath(provider, layout, step string) string {
	return filepath.Join(
		os.Getenv("MODULES_DIR"),
		cloudProviderPrefix+provider,
		"cluster-manager", // TODO  "deckhouse-cluster" or "cluster" ??
		"layouts",
		layout,
		step,
	)
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
