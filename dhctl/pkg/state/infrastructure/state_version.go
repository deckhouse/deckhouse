// Copyright 2025 Flant JSC
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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const opentofuConvertationMsg = "Terraform state detected while opentofu state was expected. Do you want to apply migration? Use with caution, and only if there are no changes in the execution plan!"

var ErrTerraformState = errors.New("Cannot converge state because state is terraform, not opentofu")

type TerraformVersions struct {
	Terraform string
	OpenTofu  string
}

var DefaultTerraformVersions = TerraformVersions{
	Terraform: "0.14.8",
	OpenTofu:  "1.9.4",
}

type State struct {
	TerraformVersion string     `json:"terraform_version"`
	Resources        []Resource `json:"resources"`
}

type Resource struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

type TerraformVersionOutput struct {
	TerraformVersion string `json:"terraform_version"`
}

type TerraformVersion struct {
	InStateVersion string `json:"in_state_version,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
}

func parseTerraformVersion(output []byte) (string, error) {
	var info TerraformVersionOutput
	if err := json.Unmarshal(output, &info); err != nil {
		return "", fmt.Errorf("cannot parse terraform version output: %w", err)
	}
	return info.TerraformVersion, nil
}

func IsTerraformState(output []byte) (bool, error) {
	parsedVersion, err := parseTerraformVersion(output)
	if err != nil {
		return false, err
	}

	log.DebugF("Terraform Version: %s\n", parsedVersion)

	res := parsedVersion == DefaultTerraformVersions.Terraform

	return res, nil
}

func CheckCanIConvergeTerraformStateWhenWeUseTofu(output []byte) error {
	isTerraform, err := IsTerraformState(output)
	if err != nil {
		return err
	}

	if isTerraform {
		return ErrTerraformState
	}

	return nil
}

func AskCanIConvergeTerraformStateWhenWeUseTofu(output []byte) error {
	isTerraform, err := IsTerraformState(output)
	if err != nil {
		return err
	}

	if isTerraform && !input.NewConfirmation().WithMessage(opentofuConvertationMsg).Ask() {
		return ErrTerraformState
	}

	return nil
}
