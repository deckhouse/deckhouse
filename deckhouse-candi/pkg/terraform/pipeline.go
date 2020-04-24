package terraform

import (
	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
)

type Pipeline struct {
	Step            string
	TerraformRunner Interface
	MetaConfig      *config.MetaConfig
	GetResult       func(*Pipeline) (map[string][]byte, error)
}

func NewPipeline(step string, metaConfig *config.MetaConfig, getResult func(*Pipeline) (map[string][]byte, error)) *Pipeline {
	tfRunner := NewRunner(step, metaConfig)
	return &Pipeline{Step: step, TerraformRunner: tfRunner, MetaConfig: metaConfig, GetResult: getResult}
}

func (p *Pipeline) runTerraform() error {
	bootstrap := p.Step != "base_infrastructure"

	out, err := p.TerraformRunner.Init(bootstrap)
	if err != nil {
		logboek.LogInfoLn(string(out))
		return err
	}

	out, err = p.TerraformRunner.Apply()
	if err != nil {
		logboek.LogInfoLn(string(out))
		return err
	}

	return nil
}

func (p *Pipeline) Run() (map[string][]byte, error) {
	var result map[string][]byte
	err := logboek.LogProcess("Run terraform pipeline "+p.Step, log.BoldOptions(), func() error {
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
	deckhouseConfig, err := p.TerraformRunner.GetTerraformOutput("deckhouse_config")
	if err != nil {
		return nil, err
	}

	cloudDiscovery, err := p.TerraformRunner.GetTerraformOutput("cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	tfState, err := p.TerraformRunner.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"terraformState":  tfState,
		"deckhouseConfig": deckhouseConfig,
		"cloudDiscovery":  cloudDiscovery,
	}, nil
}

func GetMasterPipelineResult(p *Pipeline) (map[string][]byte, error) {
	deckhouseConfig, err := p.TerraformRunner.GetTerraformOutput("deckhouse_config")
	if err != nil {
		return nil, err
	}

	masterIPAddress, err := p.TerraformRunner.GetTerraformOutput("master_ip_address")
	if err != nil {
		return nil, err
	}

	masterInstanceClass, err := p.TerraformRunner.GetTerraformOutput("master_instance_class")
	if err != nil {
		return nil, err
	}

	nodeIP, err := p.TerraformRunner.GetTerraformOutput("node_ip")
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"masterIP":            masterIPAddress,
		"nodeIP":              nodeIP,
		"deckhouseConfig":     deckhouseConfig,
		"masterInstanceClass": masterInstanceClass,
	}, nil
}
