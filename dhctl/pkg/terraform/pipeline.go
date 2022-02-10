// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package terraform

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PipelineOutputs struct {
	TerraformState []byte
	CloudDiscovery []byte

	BastionHost string

	MasterIPForSSH     string
	NodeInternalIP     string
	KubeDataDevicePath string
}

func equalArray(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
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

		defer func() { extractedData, err = extractFn(r) }()

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

func CheckPipeline(r *Runner, name string) (int, error) {
	isChange := PlanHasNoChanges
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

func CheckBaseInfrastructurePipeline(r *Runner, name string) (int, error) {
	isChange := PlanHasNoChanges
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
		if isChange > PlanHasChanges {
			return nil
		}

		info, err := GetBaseInfraResult(r)
		if err != nil {
			isChange = PlanHasDestructiveChanges
			return err
		}

		// Because terraform 0.14 is not able to track changes in outputs correctly, we have to do it in dhctl code
		// by manually comparing `zones` arrays from the plan and from the state
		var data struct {
			Zones []string `json:"zones"`
		}
		if err := json.Unmarshal(info.CloudDiscovery, &data); err != nil {
			return err
		}

		var changes struct {
			Output struct {
				Data struct {
					After struct {
						Zones []string `json:"zones"`
					} `json:"after"`
				} `json:"cloud_discovery_data"`
			} `json:"output_changes"`
		}

		result, err := r.terraformExecutor.Output("show", "-json", r.planPath)
		if err != nil {
			var ee *exec.ExitError
			if ok := errors.As(err, &ee); ok {
				err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
			}
			return fmt.Errorf("can't get terraform plan for %q\n%v", r.planPath, err)
		}

		err = json.Unmarshal(result, &changes)
		if err != nil {
			return err
		}

		sort.Strings(changes.Output.Data.After.Zones)
		sort.Strings(data.Zones)

		if !equalArray(data.Zones, changes.Output.Data.After.Zones) {
			isChange = PlanHasDestructiveChanges
		}

		return nil
	}
	err := log.Process("terraform", fmt.Sprintf("Check state %s for %s", r.step, name), pipelineFunc)
	return isChange, err
}

func DestroyPipeline(r *Runner, name string) error {
	pipelineFunc := func() error {
		err := r.Init()
		if err != nil {
			return err
		}

		if r.ResourcesQuantityInState() == 0 {
			log.InfoLn("Nothing to destroy! Skipping ...")
			return nil
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

	// bastion host is optional
	bastionHost, _ := getStringOrIntOutput(r, "bastion_ip_address_for_ssh")

	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		TerraformState: tfState,
		CloudDiscovery: cloudDiscovery,
		BastionHost:    bastionHost,
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
