package terraform

import (
	"encoding/json"
	"fmt"

	"flant/deckhouse-candi/pkg/log"
)

type PipelineOutputs struct {
	TerraformState []byte
	CloudDiscovery []byte

	MasterIPForSSH     string
	NodeInternalIP     string
	KubeDataDevicePath string
}

func ApplyPipeline(r *Runner, name string, extractFn func(r *Runner) (*PipelineOutputs, error)) (*PipelineOutputs, error) {
	var extractedData *PipelineOutputs
	pipelineFunc := func() error {
		err := r.Init()
		if err != nil {
			return err
		}

		err = r.Plan()
		if err != nil {
			return err
		}

		err = r.Apply()
		if err != nil {
			return err
		}

		extractedData, err = extractFn(r)
		return err
	}

	err := log.TerraformProcess(fmt.Sprintf("Pipeline %s for %s", r.step, name), pipelineFunc)
	return extractedData, err
}

func CheckPipeline(r *Runner) (bool, error) {
	err := r.Init()
	if err != nil {
		return false, err
	}

	err = r.Plan()
	if err != nil {
		return false, err
	}

	return r.changesInPlan, err
}

func DestroyPipeline(r *Runner, name string) error {
	pipelineFunc := func() error {
		err := r.Init()
		if err != nil {
			return err
		}

		err = r.Destroy()
		if err != nil {
			return err
		}
		return nil
	}
	return log.TerraformProcess(fmt.Sprintf("Destroy %s for %s", r.step, name), pipelineFunc)
}

func GetBaseInfraResult(r *Runner) (*PipelineOutputs, error) {
	cloudDiscovery, err := r.GetTerraformOutput("cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		TerraformState: tfState,
		CloudDiscovery: cloudDiscovery,
	}, nil
}

func GetMasterNodeResult(r *Runner) (*PipelineOutputs, error) {
	masterIPAddressForSSH, err := getStringOutput(r, "master_ip_address_for_ssh")
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := getStringOutput(r, "node_internal_ip_address")
	if err != nil {
		return nil, err
	}

	kubernetesDataDevicePath, err := getStringOutput(r, "kubernetes_data_device_path")
	if err != nil {
		return nil, err
	}

	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		TerraformState:     tfState,
		MasterIPForSSH:     masterIPAddressForSSH,
		NodeInternalIP:     nodeInternalIP,
		KubeDataDevicePath: kubernetesDataDevicePath,
	}, nil
}

func OnlyState(r *Runner) (*PipelineOutputs, error) {
	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{TerraformState: tfState}, nil
}

func getStringOutput(r *Runner, name string) (string, error) {
	outputRaw, err := r.GetTerraformOutput(name)
	if err != nil {
		return "", err
	}

	var output string
	// skip error check here, because terraform always return valid json
	_ = json.Unmarshal(outputRaw, &output)
	return output, nil
}
