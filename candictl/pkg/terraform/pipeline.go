package terraform

import (
	"encoding/json"
	"fmt"
	"strconv"

	"flant/candictl/pkg/config"
	"flant/candictl/pkg/log"
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

	err := log.Process("terraform", fmt.Sprintf("Pipeline %s for %s", r.step, name), pipelineFunc)
	return extractedData, err
}

func CheckPipeline(r *Runner, name string) (bool, error) {
	isChange := false
	pipelineFunc := func() error {
		err := r.Init()
		if err != nil {
			return err
		}

		err = r.Plan()
		if err != nil {
			return err
		}

		isChange = r.changesInPlan
		return nil
	}
	err := log.Process("terraform", fmt.Sprintf("Check state %s for %s", r.step, name), pipelineFunc)
	return isChange, err
}

func DestroyPipeline(r *Runner, name string) error {
	pipelineFunc := func() error {
		if r.ResourcesQuantityInState() == 0 {
			log.InfoLn("Nothing to destroy! Skipping ...")
			return nil
		}

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
	return log.Process("terraform", fmt.Sprintf("Destroy %s for %s", r.step, name), pipelineFunc)
}

func GetBaseInfraResult(r *Runner) (*PipelineOutputs, error) {
	cloudDiscovery, err := r.GetTerraformOutput("cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	schemaStore := config.NewSchemaStore()
	_, err = schemaStore.Validate(&cloudDiscovery)
	if err != nil {
		return nil, fmt.Errorf("validate cloud_discovery_data: %v", err)
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
	masterIPAddressForSSH, err := getStringOrIntOutput(r, "master_ip_address_for_ssh")
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := getStringOrIntOutput(r, "node_internal_ip_address")
	if err != nil {
		return nil, err
	}

	kubernetesDataDevicePath, err := getStringOrIntOutput(r, "kubernetes_data_device_path")
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

type stringOrInt string

func (s *stringOrInt) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = stringOrInt(str)
		return nil
	}

	var i int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}

	*s = stringOrInt(strconv.Itoa(i))
	return nil
}

func getStringOrIntOutput(r *Runner, name string) (string, error) {
	outputRaw, err := r.GetTerraformOutput(name)
	if err != nil {
		return "", err
	}

	var output stringOrInt
	// skip error check here, because terraform always return valid json
	_ = json.Unmarshal(outputRaw, &output)
	return string(output), nil
}
