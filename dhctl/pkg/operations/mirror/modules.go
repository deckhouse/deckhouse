// Copyright 2023 Flant JSC
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

package mirror

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Module struct {
	Name         string
	RegistryPath string
	Releases     []string
}

func GetExternalModules(mirrorCtx *Context) ([]Module, error) {
	nameOpts := []name.Option{}
	remoteOpts := []remote.Option{}
	if mirrorCtx.Insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	if mirrorCtx.RegistryAuth != nil {
		remoteOpts = append(remoteOpts, remote.WithAuth(mirrorCtx.RegistryAuth))
	}

	modulesRepo, err := name.NewRepository(mirrorCtx.DeckhouseRegistryRepo+"/modules", nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing modules repo: %v", err)
	}

	modules, err := remote.List(modulesRepo, remote.WithAuth(mirrorCtx.RegistryAuth))
	if err != nil {
		return nil, fmt.Errorf("read Deckhouse modules from registry: %w", err)
	}

	result := make([]Module, 0, len(modules))
	for _, module := range modules {
		m := Module{
			Name:         module,
			RegistryPath: fmt.Sprintf("%s/modules/%s", mirrorCtx.DeckhouseRegistryRepo, module),
			Releases:     []string{},
		}

		repo, err := name.NewRepository(m.RegistryPath+"/release", nameOpts...)
		if err != nil {
			return nil, fmt.Errorf("parsing repo: %v", err)
		}
		m.Releases, err = remote.List(repo, remoteOpts...)
		if err != nil {
			return nil, fmt.Errorf("get releases for module %q: %w", m.RegistryPath, err)
		}
		result = append(result, m)
	}

	return result, nil
}
