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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

func (b *ClusterBootstrapper) CreateResources(ctx context.Context) error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	resourcesToCreate := make(template.Resources, 0)
	if app.ResourcesPath != "" {
		log.WarnLn("--resources flag is deprecated. Please use --config flag multiple repeatedly for logical resources separation")
		parsedResources, err := template.ParseResources(app.ResourcesPath, nil)
		if err != nil {
			return err
		}

		resourcesToCreate = parsedResources
	} else {
		paths := fs.RevealWildcardPaths(app.ConfigPaths)
		for _, cfg := range paths {
			ress, err := template.ParseResources(cfg, nil)
			if err != nil {
				return err
			}

			resourcesToCreate = append(resourcesToCreate, ress...)
		}
	}

	log.DebugF("Resources: %s\n", resourcesToCreate.String())

	if len(resourcesToCreate) == 0 {
		log.WarnLn("Resources to create were not found.")
		return nil
	}

	if err := terminal.AskBecomePassword(); err != nil {
		return err
	}
	if err := terminal.AskBastionPassword(); err != nil {
		return err
	}

	if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok && wrapper != nil {
		sshClient := wrapper.Client()
		if sshClient != nil {
			if err := sshClient.Start(); err != nil {
				return fmt.Errorf("unable to start ssh-client: %w", err)
			}
		}
	}

	return log.Process("bootstrap", "Create resources", func() error {
		kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, b.NodeInterface)
		if err != nil {
			return err
		}

		checkers, err := resources.GetCheckers(kubeCl, resourcesToCreate, nil)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(ctx, kubeCl, resourcesToCreate, checkers, nil)
	})
}
