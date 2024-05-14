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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func (b *ClusterBootstrapper) CreateResources() error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	var resourcesToCreate template.Resources
	if app.ResourcesPath != "" {
		parsedResources, err := template.ParseResources(app.ResourcesPath, nil)
		if err != nil {
			return err
		}

		resourcesToCreate = parsedResources
	}

	if len(resourcesToCreate) == 0 {
		log.WarnLn("Resources to create were not found.")
		return nil
	}

	if b.SSHClient != nil {
		if _, err := b.SSHClient.Start(); err != nil {
			return fmt.Errorf("unable to start ssh client: %w", err)
		}
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
	}

	return log.Process("bootstrap", "Create resources", func() error {
		kubeCl, err := operations.ConnectToKubernetesAPI(b.SSHClient)
		if err != nil {
			return err
		}

		checkers, err := resources.GetCheckers(kubeCl, resourcesToCreate, nil)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(kubeCl, resourcesToCreate, checkers)
	})
}
