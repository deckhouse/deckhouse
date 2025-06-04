/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package deckhouseregistry

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

type Params struct {
	RegistrySecret deckhouse_registry.Config

	ManagedMode   *ManagedModeParams
	UnmanagedMode *UnmanagedModeParams
}

type ManagedModeParams struct {
	CA       string
	Username string
	Password string
}

type UnmanagedModeParams struct {
	ImagesRegistry string
	Scheme         string
	CA             string
	Username       string
	Password       string
}

type State struct {
	Config deckhouse_registry.Config `json:"-"`
}

func (s *State) Process(params Params) (bool, error) {
	newSecret, err := buildRegistrySecret(params)
	if err != nil {
		return false, fmt.Errorf("cannot build deckhouse-registry secret: %w", err)
	}
	s.Config = newSecret

	if newSecret.Equal(&params.RegistrySecret) {
		return true, nil
	}
	return false, nil
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
	address, path := getRegistryAddressAndPathFromImagesRepo(params.ImagesRegistry)

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
