/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

import (
	"fmt"
	"strings"
	"time"

	gcr_name "github.com/google/go-containerregistry/pkg/name"
)

type deckhouseImagesModel struct {
	InitContainers map[string]string
	Containers     map[string]string
}

type queueItem struct {
	Image string `json:"image,omitempty"`
	Info  string `json:"info,omitempty"`
	Error string `json:"error,omitempty"`
}

type Params struct {
	Registries map[string]RegistryParams `json:"registries,omitempty"`
	Hash       string                    `json:"hash,omitempty"`
}

type RegistryParams struct {
	Address  string `json:"address,omitempty"`
	Scheme   string `json:"scheme,omitempty"`
	CA       string `json:"ca,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (r *RegistryParams) toGCRepo() (gcr_name.Repository, error) {
	var opts []gcr_name.Option

	if strings.ToUpper(r.Scheme) == "HTTP" {
		opts = append(opts, gcr_name.Insecure)
	}

	return gcr_name.NewRepository(r.Address, opts...)
}

type State struct {
	Queues map[string]RegistryQueue `json:"queues,omitempty"`
	Hash   string                   `json:"hash,omitempty"`
}

type RegistryQueue struct {
	Processed   int64       `json:"processed,omitempty"`
	Items       []queueItem `json:"items,omitempty"`
	Retry       []queueItem `json:"retry,omitempty"`
	LastAttempt *time.Time  `json:"last_attempt,omitempty"`
}

type Inputs struct {
	Params Params

	ImagesInfo  clusterImagesInfo
	Parallelizm parallelizmModel
}

type clusterImagesInfo struct {
	Repo                 string
	ModulesImagesDigests map[string]string
	DeckhouseImages      deckhouseImagesModel
}

type parallelizmModel struct {
	Total       int
	PerRegistry int
}

func (state *State) process(inputs Inputs) error {
	if state.Queues == nil {
		state.Queues = make(map[string]RegistryQueue)
	}

	for name, registryParams := range inputs.Params.Registries {
		repo, err := registryParams.toGCRepo()
		if err != nil {
			return fmt.Errorf("cannot parse registry %q params: %w", name, err)
		}

		repoImages, err := buildRepoQueue(inputs.ImagesInfo, repo)
		if err != nil {
			return fmt.Errorf("cannot collect registry %q images: %w", name, err)
		}

		q := RegistryQueue{
			Items: repoImages,
		}

		state.Queues[name] = q
	}

	return nil
}
