package terraform

import (
	"fmt"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/log"
)

type Pipeline struct {
	Step            string
	TerraformRunner Interface
	GetResult       func(*Pipeline) (map[string][]byte, error)
}

type PipelineOptions struct {
	Provider string
	Layout   string
	Step     string

	StateDir           string
	StateSuffix        string
	TerraformVariables []byte
	GetResult          func(*Pipeline) (map[string][]byte, error)
}

func NewPipeline(options *PipelineOptions) *Pipeline {
	tfRunner := NewRunner(options.Provider, options.Layout, options.Step, options.TerraformVariables).
		WithStateDir(options.StateDir).
		WithStateSuffix(options.StateSuffix)
	return &Pipeline{Step: options.Step, TerraformRunner: tfRunner, GetResult: options.GetResult}
}

func (p *Pipeline) runTerraform() error {
	if err := p.TerraformRunner.Init(); err != nil {
		return err
	}

	out, err := p.TerraformRunner.Apply()
	if err != nil {
		logboek.LogInfoLn(string(out))
		return err
	}

	return nil
}

func (p *Pipeline) Run() (map[string][]byte, error) {
	var result map[string][]byte
	err := logboek.LogProcess(fmt.Sprintf("🌳 Run Terraform pipeline %s 🌳", p.Step), log.BoldOptions(), func() error {
		err := p.runTerraform()
		if err != nil {
			return err
		}
		result, err = p.GetResult(p)
		return err
	})
	return result, err
}

func GetBasePipelineResult(p *Pipeline) (map[string][]byte, error) {
	cloudDiscovery, err := p.TerraformRunner.GetTerraformOutput("cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	tfState, err := p.TerraformRunner.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"terraformState": tfState,
		"cloudDiscovery": cloudDiscovery,
	}, nil
}

func GetMasterNodePipelineResult(p *Pipeline) (map[string][]byte, error) {
	masterIPAddressForSSH, err := p.TerraformRunner.GetTerraformOutput("master_ip_address_for_ssh")
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := p.TerraformRunner.GetTerraformOutput("node_internal_ip_address")
	if err != nil {
		return nil, err
	}

	tfState, err := p.TerraformRunner.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"terraformState": tfState,
		"masterIPForSSH": masterIPAddressForSSH,
		"nodeInternalIP": nodeInternalIP,
	}, nil
}

func OnlyState(p *Pipeline) (map[string][]byte, error) {
	tfState, err := p.TerraformRunner.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{"terraformState": tfState}, nil
}
