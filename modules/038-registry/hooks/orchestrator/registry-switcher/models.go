/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registryswitcher

import (
	"errors"
	"fmt"
	"strings"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

var (
	failedResult = Result{
		Ready:   false,
		Message: "Failed to switch....",
	}
)

type Params struct {
	RegistrySecret deckhouse_registry.Config

	ManagedMode   *ManagedModeParams
	UnmanagedMode *UnmanagedModeParams
}

type Inputs struct {
	DeckhousePod DeckhousePodStatus
}

type DeckhousePodStatus struct {
	IsExist         bool
	IsReady         bool
	ReadyMsg        string
	RegistryVersion string
}

type ManagedModeParams struct {
	CA       string
	Username string
	Password string
}

type UnmanagedModeParams struct {
	ImagesRepo string
	Scheme     string
	CA         string
	Username   string
	Password   string
}

type State struct {
	Config deckhouse_registry.Config
	Hash   string
}

type Result struct {
	Ready   bool
	Message string
}

func (s *State) Process(params Params, inputs Inputs) (Result, error) {
	newSecret, err := buildRegistrySecret(params)
	if err != nil {
		return failedResult, fmt.Errorf("cannot build deckhouse-registry secret: %w", err)
	}
	s.Config = newSecret

	hash, err := newSecret.Hash()
	if err != nil {
		return failedResult, fmt.Errorf("failed to calculate registry config hash: %w", err)
	}

	// Store calculated hash inside state for external tests / reuse
	s.Hash = hash
	return s.processResult(params, inputs), nil
}

func (s *State) processResult(params Params, inputs Inputs) Result {
	// First check if secret is ready
	secretReady := s.Config.Equal(&params.RegistrySecret)
	if !secretReady {
		return Result{
			Ready:   false,
			Message: "Updating registry for deckhouse components",
		}
	}

	// Compare applied registry version
	if inputs.DeckhousePod.RegistryVersion != s.Hash {
		return Result{
			Ready:   false,
			Message: "Applying new registry to deckhouse-controller",
		}
	}

	// Check pod exist
	if !inputs.DeckhousePod.IsExist {
		return Result{
			Ready:   false,
			Message: "Waiting for deckhouse-controller pod",
		}
	}

	// Check pod ready
	if !inputs.DeckhousePod.IsReady {
		return Result{
			Ready:   false,
			Message: "Waiting for deckhouse-controller to become ready",
		}
	}

	return Result{
		Ready: true,
	}
}

func buildRegistrySecret(params Params) (deckhouse_registry.Config, error) {
	switch {
	case params.ManagedMode != nil:
		return buildManagedRegistrySecret(params.ManagedMode)
	case params.UnmanagedMode != nil:
		return buildUnmanagedRegistrySecret(params.UnmanagedMode)
	default:
		return deckhouse_registry.Config{}, errors.New("either ManagedMode or UnmanagedMode must be provided")
	}
}

func buildManagedRegistrySecret(params *ManagedModeParams) (deckhouse_registry.Config, error) {
	dockerCfg, err := helpers.DockerCfgFromCreds(params.Username, params.Password, registry_const.Host)
	if err != nil {
		return deckhouse_registry.Config{}, fmt.Errorf("failed to create Docker config in managed mode: %w", err)
	}

	return deckhouse_registry.Config{
		Address:      registry_const.Host,
		Path:         registry_const.Path,
		Scheme:       registry_const.Scheme,
		CA:           params.CA,
		DockerConfig: dockerCfg,
	}, nil
}

func buildUnmanagedRegistrySecret(params *UnmanagedModeParams) (deckhouse_registry.Config, error) {
	address, path := helpers.RegistryAddressAndPathFromImagesRepo(params.ImagesRepo)

	dockerCfg, err := helpers.DockerCfgFromCreds(params.Username, params.Password, address)
	if err != nil {
		return deckhouse_registry.Config{}, fmt.Errorf("failed to create Docker config in unmanaged mode: %w", err)
	}

	return deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(params.Scheme),
		CA:           params.CA,
		DockerConfig: dockerCfg,
	}, nil
}
