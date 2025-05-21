/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package imagechecker

import (
	gcr "github.com/google/go-containerregistry/pkg/name"
)

type deckhouseImagesModel struct {
	InitContainers map[string]gcr.Reference
	Containers     map[string]gcr.Reference
}

type Params struct {
	Repositories map[string]RepositoryParams `json:"repositories,omitempty"`
	Hash         string                      `json:"hash,omitempty"`
}

type RepositoryParams struct {
	Address  string `json:"address,omitempty"`
	CA       string `json:"ca,omitempty"`
	UserName string `json:"user_name,omitempty"`
	Password string `json:"password,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

type State struct {
	Hash         string                     `json:"hash,omitempty"`
	Repositories map[string]RepositoryState `json:"repositories,omitempty"`
}

type RepositoryState struct {
	Ping bool
}
