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

package bootstrap

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (b *ClusterBootstrapper) CreateResources(ctx context.Context) error {
	resourcesToCreate := make(template.Resources, 0)
	if b.Options.Bootstrap.ResourcesPath != "" {
		dhlog.FromContext(ctx).WarnContext(ctx, "--resources flag is deprecated. Please use the --config flag multiple times for logical resource separation")
		parsedResources, err := template.ParseResources(ctx, b.Options.Bootstrap.ResourcesPath, nil)
		if err != nil {
			return err
		}

		resourcesToCreate = parsedResources
	} else {
		paths := fs.RevealWildcardPaths(b.Options.Global.ConfigPaths)
		for _, cfg := range paths {
			ress, err := template.ParseResources(ctx, cfg, nil)
			if err != nil {
				return err
			}

			resourcesToCreate = append(resourcesToCreate, ress...)
		}
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Resources: %s", resourcesToCreate.String()))

	if len(resourcesToCreate) == 0 {
		dhlog.FromContext(ctx).WarnContext(ctx, "No resources to create were found.")
		return nil
	}

	body := func(_ chan phases.Progress) error {
		return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create resources", func(ctx context.Context) error {
			kubeCl, err := b.KubeProvider.Client(ctx)
			if err != nil {
				return err
			}

			checkers, err := resources.GetCheckers(ctx, &client.KubernetesClient{KubeClient: kubeCl}, resourcesToCreate, nil)
			if err != nil {
				return err
			}

			return resources.CreateResourcesLoop(ctx, &client.KubernetesClient{KubeClient: kubeCl}, resourcesToCreate, checkers, nil, b.Options.Bootstrap.ResourcesTimeout)
		})
	}

	interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	if interactive {
		return runProgress(ctx, dhlog.FromContext(ctx), "Create resources", body)
	}

	return body(nil)
}
