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

package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PipelineOutputs struct {
	InfrastructureState []byte
	CloudDiscovery      []byte

	BastionHost string

	MasterIPForSSH               string
	NodeInternalIP               string
	KubeDataDevicePath           string
	SystemRegistryDataDevicePath string
}

type DataDevices struct {
	KubeDataDevicePath           string
	SystemRegistryDataDevicePath string
}

func (out *PipelineOutputs) GetDataDevices() DataDevices {
	return DataDevices{
		KubeDataDevicePath: out.KubeDataDevicePath,
		SystemRegistryDataDevicePath: out.SystemRegistryDataDevicePath,
	}
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

func GetMasterIPAddressForSSH(ctx context.Context, statePath string, executor OutputExecutor) (string, error) {
	result, err := executor.Output(ctx, statePath, "master_ip_address_for_ssh")

	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
		}

		return "", fmt.Errorf("failed to get infrastructure output for 'master_ip_address_for_ssh'\n%v", err)
	}

	var output string

	err = json.Unmarshal(result, &output)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal infrastructure output for 'master_ip_address_for_ssh'\n%v", err)
	}

	return output, nil
}

func ApplyPipeline(
	ctx context.Context,
	r RunnerInterface,
	name string,
	extractFn func(ctx context.Context, r RunnerInterface) (*PipelineOutputs, error),
) (*PipelineOutputs, error) {
	var extractedData *PipelineOutputs
	pipelineFunc := func() error {
		err := r.Init(ctx)
		if err != nil {
			return err
		}

		err = r.Plan(ctx, false)
		if err != nil {
			return err
		}

		defer func() { extractedData, err = extractFn(ctx, r) }()

		err = r.Apply(ctx)
		if err != nil {
			return err
		}

		extractedData, err = extractFn(ctx, r)
		return err
	}

	logger := r.GetLogger()
	err := logger.LogProcess("infrastructure", fmt.Sprintf("Pipeline %s for %s", r.GetStep(), name), pipelineFunc)
	return extractedData, err
}

func CheckPipeline(
	ctx context.Context,
	r RunnerInterface,
	name string,
	destroy bool,
) (int, plan.Plan, *plan.DestructiveChanges, error) {
	isChange := plan.HasNoChanges
	var destructiveChanges *plan.DestructiveChanges
	var infrastructurePlan map[string]any

	pipelineFunc := func() error {
		err := r.Init(ctx)
		if err != nil {
			return err
		}

		err = r.Plan(ctx, destroy)
		if err != nil {
			return err
		}

		isChange = r.GetChangesInPlan()
		destructiveChanges = r.GetPlanDestructiveChanges()

		rawPlan, err := r.ShowPlan(ctx)
		if err != nil {
			return err
		}

		err = json.Unmarshal(rawPlan, &infrastructurePlan)
		if err != nil {
			return err
		}

		return nil
	}
	err := log.Process("infrastructure", fmt.Sprintf("Check state %s for %s", r.GetStep(), name), pipelineFunc)
	return isChange, infrastructurePlan, destructiveChanges, err
}

type BaseInfrastructureDestructiveChanges struct {
	plan.DestructiveChanges
	OutputBrokenReason string           `json:"output_broken_reason,omitempty"`
	OutputZonesChanged plan.ValueChange `json:"output_zones_changed,omitempty"`
}

func CheckBaseInfrastructurePipeline(
	ctx context.Context,
	r RunnerInterface,
	name string,
) (int, plan.Plan, *BaseInfrastructureDestructiveChanges, error) {
	isChange := plan.HasNoChanges

	var destructiveChanges *BaseInfrastructureDestructiveChanges
	getOrCreateDestructiveChanges := func() *BaseInfrastructureDestructiveChanges {
		if destructiveChanges == nil {
			destructiveChanges = &BaseInfrastructureDestructiveChanges{}
		}
		return destructiveChanges
	}
	var pl map[string]any

	pipelineFunc := func() error {
		err := r.Init(ctx)
		if err != nil {
			return err
		}

		err = r.Plan(ctx, false)
		if err != nil {
			return err
		}

		isChange = r.GetChangesInPlan()
		if pdc := r.GetPlanDestructiveChanges(); pdc != nil {
			getOrCreateDestructiveChanges().DestructiveChanges = *pdc
		}
		if isChange > plan.HasChanges {
			return nil
		}

		info, err := GetBaseInfraResult(ctx, r)
		if err != nil {
			isChange = plan.HasDestructiveChanges
			getOrCreateDestructiveChanges().OutputBrokenReason = err.Error()
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

		rawPlan, err := r.ShowPlan(ctx)
		if err != nil {
			return err
		}

		err = json.Unmarshal(rawPlan, &changes)
		if err != nil {
			return err
		}

		err = json.Unmarshal(rawPlan, &pl)
		if err != nil {
			return err
		}

		sort.Strings(changes.Output.Data.After.Zones)
		sort.Strings(data.Zones)

		if !equalArray(data.Zones, changes.Output.Data.After.Zones) {
			isChange = plan.HasDestructiveChanges
			getOrCreateDestructiveChanges().OutputZonesChanged = plan.ValueChange{
				CurrentValue: data.Zones,
				NextValue:    changes.Output.Data.After.Zones,
			}
		}

		return nil
	}
	err := log.Process("infrastructure", fmt.Sprintf("Check state %s for %s", r.GetStep(), name), pipelineFunc)
	return isChange, pl, destructiveChanges, err
}

func DestroyPipeline(ctx context.Context, r RunnerInterface, name string) error {
	pipelineFunc := func() error {
		err := r.Init(ctx)
		if err != nil {
			return err
		}

		if r.ResourcesQuantityInState() == 0 {
			log.InfoLn("Nothing to destroy! Skipping ...")
			return nil
		}

		err = r.Destroy(ctx)
		if err != nil {
			return err
		}
		return nil
	}
	return log.Process("infrastructure", fmt.Sprintf("Destroy %s for %s", r.GetStep(), name), pipelineFunc)
}

func GetBaseInfraResult(ctx context.Context, r RunnerInterface) (*PipelineOutputs, error) {
	cloudDiscovery, err := r.GetInfrastructureOutput(ctx, "cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	schemaStore := config.NewSchemaStore()
	_, err = schemaStore.Validate(&cloudDiscovery)
	if err != nil {
		return nil, fmt.Errorf("validate cloud_discovery_data: %v", err)
	}

	// bastion host is optional
	bastionHost, _ := getStringOrIntOutput(ctx, r, "bastion_ip_address_for_ssh")

	tfState, err := r.GetState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		InfrastructureState: tfState,
		CloudDiscovery:      cloudDiscovery,
		BastionHost:         bastionHost,
	}, nil
}

func GetMasterNodeResult(ctx context.Context, r RunnerInterface) (*PipelineOutputs, error) {
	masterIPAddressForSSH, err := getStringOrIntOutput(ctx, r, "master_ip_address_for_ssh")
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := getStringOrIntOutput(ctx, r, "node_internal_ip_address")
	if err != nil {
		return nil, err
	}

	kubernetesDataDevicePath, err := getStringOrIntOutput(ctx, r, "kubernetes_data_device_path")
	if err != nil {
		return nil, err
	}

	systemRegistryDataDevicePath, err := getStringOrIntOutput(ctx, r, "system_registry_data_device_path")
	if err != nil {
		return nil, err
	}

	tfState, err := r.GetState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		InfrastructureState:          tfState,
		MasterIPForSSH:               masterIPAddressForSSH,
		NodeInternalIP:               nodeInternalIP,
		KubeDataDevicePath:           kubernetesDataDevicePath,
		SystemRegistryDataDevicePath: systemRegistryDataDevicePath,
	}, nil
}

func OnlyState(_ context.Context, r RunnerInterface) (*PipelineOutputs, error) {
	tfState, err := r.GetState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{InfrastructureState: tfState}, nil
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

func getStringOrIntOutput(ctx context.Context, r RunnerInterface, name string) (string, error) {
	outputRaw, err := r.GetInfrastructureOutput(ctx, name)
	if err != nil {
		return "", err
	}

	var output stringOrInt
	// skip error check here, because infra utility always return valid json
	_ = json.Unmarshal(outputRaw, &output)
	return string(output), nil
}
