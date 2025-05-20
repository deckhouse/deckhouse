/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package deckhouseregistry

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

type Params struct {
	RegistrySecret deckhouse_registry.Secret

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

func Process(params Params, patchCollector go_hook.PatchCollector) (bool, error) {
	newSecret, err := buildRegistrySecret(params)
	if err != nil {
		return false, fmt.Errorf("cannot build deckhouse-registry secret: %w", err)
	}

	if newSecret.Equal(&params.RegistrySecret) {
		return true, nil
	}

	patchPayload := map[string]interface{}{
		"data": map[string]string{
			".dockerconfigjson": base64.StdEncoding.EncodeToString(newSecret.DockerConfig),
			"address":           base64.StdEncoding.EncodeToString([]byte(newSecret.Address)),
			"imagesRegistry":    base64.StdEncoding.EncodeToString([]byte(newSecret.ImagesRegistry)),
			"path":              base64.StdEncoding.EncodeToString([]byte(newSecret.Path)),
			"ca":                base64.StdEncoding.EncodeToString([]byte(newSecret.CA)),
			"scheme":            base64.StdEncoding.EncodeToString([]byte(newSecret.Scheme)),
		},
	}
	patchCollector.PatchWithMerge(patchPayload, "v1", "Secret", "d8-system", "deckhouse-registry")
	return false, nil
}

func buildRegistrySecret(params Params) (deckhouse_registry.Secret, error) {
	switch {
	case params.ManagedMode != nil:
		return buildManagedRegistrySecret(params.ManagedMode)
	case params.UnmanagedMode != nil:
		return buildUnmanagedRegistrySecret(params.UnmanagedMode)
	default:
		return deckhouse_registry.Secret{}, errors.New("either ManagedMode or UnmanagedMode must be provided")
	}
}

func buildManagedRegistrySecret(params *ManagedModeParams) (deckhouse_registry.Secret, error) {
	dockerCfg, err := helpers.DockerCfgFromCreds(params.Username, params.Password, registry_const.Host)
	if err != nil {
		return deckhouse_registry.Secret{}, fmt.Errorf("failed to create Docker config in managed mode: %w", err)
	}

	return deckhouse_registry.Secret{
		Address:        registry_const.Host,
		Path:           registry_const.Path,
		Scheme:         registry_const.Scheme,
		ImagesRegistry: registry_const.HostWithPath,
		CA:             params.CA,
		DockerConfig:   dockerCfg,
	}, nil
}

func buildUnmanagedRegistrySecret(params *UnmanagedModeParams) (deckhouse_registry.Secret, error) {
	address, path := getRegistryAddressAndPathFromImagesRepo(params.ImagesRegistry)

	dockerCfg, err := helpers.DockerCfgFromCreds(params.Username, params.Password, address)
	if err != nil {
		return deckhouse_registry.Secret{}, fmt.Errorf("failed to create Docker config in unmanaged mode: %w", err)
	}

	return deckhouse_registry.Secret{
		Address:        address,
		Path:           path,
		Scheme:         strings.ToLower(params.Scheme),
		CA:             params.CA,
		ImagesRegistry: params.ImagesRegistry,
		DockerConfig:   dockerCfg,
	}, nil
}
