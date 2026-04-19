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
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	masterSSHIPOutputKey    = "master_ip_address_for_ssh"
	nodeInternalIPOutputKey = "node_internal_ip_address"
	kubeDataPathOutputKey   = "kubernetes_data_device_path"
)

type PipelineOutputs struct {
	InfrastructureState []byte
	CloudDiscovery      []byte

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

type OutputMasterIPs struct {
	SSH      string
	Internal string
}

func GetMasterIPAddressForSSH(ctx context.Context, statePath string, executor OutputExecutor) (*OutputMasterIPs, error) {
	res := OutputMasterIPs{}

	outputs := map[string]*string{
		masterSSHIPOutputKey:    &res.SSH,
		nodeInternalIPOutputKey: &res.Internal,
	}

	for k, v := range outputs {
		result, err := executor.Output(ctx, OutputOpts{
			StatePath: statePath,
			OutFields: []string{k},
		})

		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				err = fmt.Errorf("%s\n%v", string(ee.Stderr), err)
			}
			if matchNoOutput(err.Error()) {
				*v = ""
				continue
			}

			return nil, fmt.Errorf("Cannot extract infrastructure output for '%s': %w", k, err)
		}

		var output string

		err = json.Unmarshal(result, &output)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal infrastructure output for '%s': %w", k, err)
		}

		*v = output
	}

	return &res, nil
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

		err = r.Plan(ctx, false, false)
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
	noout bool,
) (int, plan.Plan, *plan.DestructiveChanges, error) {
	isChange := plan.HasNoChanges
	var destructiveChanges *plan.DestructiveChanges
	var infrastructurePlan map[string]any

	pipelineFunc := func() error {
		err := r.Init(ctx)
		if err != nil {
			return err
		}

		err = r.Plan(ctx, destroy, noout)
		if err != nil {
			return err
		}

		isChange = r.GetChangesInPlan()
		if noout {
			return nil
		}
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

	logDebugPlanIfNeed(ctx, r, name, destroy)

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

		err = r.Plan(ctx, false, false)
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

	logDebugPlanIfNeed(ctx, r, name, false)

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

	schemaStore := config.NewSchemaStore(nil)
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
	masterIPAddressForSSH, err := getStringOrIntOutput(ctx, r, masterSSHIPOutputKey)
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := getStringOrIntOutput(ctx, r, nodeInternalIPOutputKey)
	if err != nil {
		return nil, err
	}

	kubernetesDataDevicePath, err := getStringOrIntOutput(ctx, r, kubeDataPathOutputKey)
	if err != nil {
		return nil, err
	}

	tfState, err := r.GetState()
	if err != nil {
		return nil, err
	}

	return &PipelineOutputs{
		InfrastructureState: tfState,
		MasterIPForSSH:      masterIPAddressForSSH,
		NodeInternalIP:      nodeInternalIP,
		KubeDataDevicePath:  kubernetesDataDevicePath,
	}, nil
}

// GetMasterNodeResultNoStrict
// set to empty if any output is not present
// if state not exists or empty returns empty
// if incorrect state returns error
func GetMasterNodeResultNoStrict(ctx context.Context, r RunnerInterface) (*PipelineOutputs, error) {
	res := &PipelineOutputs{}
	toReceive := map[string]*string{
		masterSSHIPOutputKey:    &res.MasterIPForSSH,
		nodeInternalIPOutputKey: &res.NodeInternalIP,
		kubeDataPathOutputKey:   &res.KubeDataDevicePath,
	}

	for k, dest := range toReceive {
		output, err := getStringOrIntOutput(ctx, r, k)
		if err != nil {
			if matchNoOutput(err.Error()) {
				*dest = ""
				continue
			}
			return nil, err
		}

		*dest = output
	}

	tfState, err := r.GetState()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		tfState = make([]byte, 0)
	}

	res.InfrastructureState = tfState

	return res, nil
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

var noOutputRegexps = []*regexp.Regexp{
	// tofu
	regexp.MustCompile(`Output ".+" not found`),
	// terraform
	regexp.MustCompile(`The output variable requested could not be found in the state`),
}

func matchNoOutput(err string) bool {
	for _, re := range noOutputRegexps {
		if re.MatchString(err) {
			return true
		}
	}

	return false
}

func logDebugPlanIfNeed(ctx context.Context, r RunnerInterface, name string, destroy bool) {
	const (
		stepEnv = "DHCTL_CLI_DEBUG_PLAN_STEP"
		// targetEnv
		// should be
		// module.$STEP.resource.resource_name like
		// module.static-node.kubernetes_manifest.vm
		// separated by ;
		targetsEnv = "DHCTL_CLI_DEBUG_PLAN_TARGETS"
	)

	debugPlanStep := os.Getenv(stepEnv)
	debugPlanTargetsStr := os.Getenv(targetsEnv)

	targetsStr := fmt.Sprintf("%s - %s: %s", name, debugPlanStep, debugPlanTargetsStr)

	skipMessage := func(f string, args ...any) string {
		m := fmt.Sprintf(f, args...)
		return fmt.Sprintf("Skip debug plan for %s: %s", targetsStr, m)
	}

	skipDebug := func(f string, args ...any) {
		log.DebugF("%s\n", skipMessage(f, args...))
	}

	skipInfo := func(f string, args ...any) {
		log.InfoF("%s\n", skipMessage(f, args...))
	}

	if debugPlanStep == "" {
		skipDebug("step env %s not set", stepEnv)
		return
	}

	targetsRaw := strings.Split(debugPlanTargetsStr, ";")
	targets := make([]string, 0, len(targetsRaw))
	for _, target := range targetsRaw {
		t := strings.TrimSpace(target)
		if t != "" {
			targets = append(targets, t)
		}
	}

	if len(targets) == 0 {
		skipDebug("pass empty targets with env %s", targetsEnv)
		return
	}

	if destroy {
		skipInfo("no out destroy plan, because it is produce only destroy changes")
		return
	}

	executorStep := string(r.GetStep())

	if debugPlanStep != executorStep {
		skipInfo("passed step %s not match with executor step %s", debugPlanStep, executorStep)
		return
	}

	results := make(map[string]string, len(targets))
	resultsErrs := make(map[string]error, len(targets))

	// always return nil
	_ = log.Process("infrastructure", "Getting debug plans", func() error {
		for _, target := range targets {
			res, err := r.DebugPlanTarget(ctx, destroy, debugPlanStep, target)
			if err != nil {
				resultsErrs[target] = err
				continue
			}
			results[target] = res
		}

		return nil
	})

	targetStr := func(t string) string {
		return fmt.Sprintf("%s - %s/%s", name, debugPlanStep, t)
	}

	for target, targetErr := range resultsErrs {
		log.WarnF("Cannot get plan output for %s: %v\n", targetStr(target), targetErr)
	}

	for target, output := range results {
		fullPretty, forTarget := extractChangesStrings(target, output)
		log.DebugF("Full debug output plan for %s:\n%s\n", targetStr(target), fullPretty)
		log.InfoF("Changes in plan for %s:\n%s\n", targetStr(target), forTarget)
	}
}

func extractChangesStrings(target string, planOutput string) (string, string) {
	var mapOut map[string]any
	err := json.Unmarshal([]byte(planOutput), &mapOut)
	if err != nil {
		return planOutput, ""
	}

	changesForTarget := extractChanges(target, mapOut)

	prettyOutput, err := json.MarshalIndent(mapOut, "", "  ")
	if err != nil {
		return planOutput, changesForTarget
	}

	return string(prettyOutput), changesForTarget
}

func extractChanges(target string, mapOut map[string]any) string {
	changesRaw, ok := mapOut["resource_changes"]
	if !ok {
		return "Plan does not contain resource_changes key"
	}

	changes, ok := changesRaw.([]any)
	if !ok {
		return fmt.Sprintf("Plan resource_changes key is not []any it is %T", changesRaw)
	}

	for i, changeRaw := range changes {
		change, ok := changeRaw.(map[string]any)
		if !ok {
			msg := fmt.Sprintf("Plan resource_changes key index %d for %s is not map[string]any it is %T", i, target, changesRaw)
			log.DebugF("%s\n", msg)
			continue
		}

		address, ok := change["address"]
		if !ok {
			msg := fmt.Sprintf("Plan resource_changes key index %d for %s does not contain address key", i, target)
			log.DebugF("%s\n", msg)
			continue
		}

		addressStr, ok := address.(string)
		if !ok {
			msg := fmt.Sprintf("Plan resource_changes key index %d for %s address is not string it is %T", i, target, address)
			log.DebugF("%s\n", msg)
			continue
		}

		if addressStr != target {
			continue
		}

		changeBytes, err := json.MarshalIndent(change, "", "  ")
		if err != nil {
			return fmt.Sprintf("Cannot marshal changes: %v", err)
		}

		return string(changeBytes)
	}

	return "Changes not found"
}
