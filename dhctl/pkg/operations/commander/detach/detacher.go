// Copyright 2024 Flant JSC
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

package detach

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type DetacherOptions struct {
	DeleteDetachResources DetachResources
	CreateDetachResources DetachResources
	OnCheckResult         func(*check.CheckResult) error
}

type Detacher struct {
	DetacherOptions

	AgentModuleName string
	SSHClient       *ssh.Client
	Checker         *check.Checker
}

type DetachResources struct {
	Template string
	Values   map[string]any
}

func NewDetacher(checker *check.Checker, sshClient *ssh.Client, opts DetacherOptions) *Detacher {
	return &Detacher{
		DetacherOptions: opts,
		SSHClient:       sshClient,
		Checker:         checker,
	}
}

func (op *Detacher) Detach(ctx context.Context) error {
	err := log.Process("commander/detach", "Check cluster", func() error {
		checkRes, err := op.Checker.Check(ctx)
		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}

		if op.OnCheckResult != nil {
			if err := op.OnCheckResult(checkRes); err != nil {
				return fmt.Errorf("oncheckResult callback failed: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = log.Process("commander/detach", "Create resources", func() error {
		detachResources, err := template.ParseResourcesContent(
			op.CreateDetachResources.Template,
			op.CreateDetachResources.Values,
		)
		if err != nil {
			return fmt.Errorf("unable to parse resources to create: %w", err)
		}

		kubeClient, err := op.Checker.GetKubeClient()
		if err != nil {
			return fmt.Errorf("unable to get kube client: %w", err)
		}

		checkers, err := resources.GetCheckers(kubeClient, detachResources, nil)
		if err != nil {
			return fmt.Errorf("unable to get resource checkers: %w", err)
		}

		err = resources.CreateResourcesLoop(kubeClient, detachResources, checkers)
		if err != nil {
			return fmt.Errorf("unable to create resources: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = log.Process("commander/detach", "Remove commander resources", func() error {
		detachResources, err := template.ParseResourcesContent(
			op.DeleteDetachResources.Template,
			op.DeleteDetachResources.Values,
		)
		if err != nil {
			return fmt.Errorf("unable to parse resources to delete: %w", err)
		}

		kubeClient, err := op.Checker.GetKubeClient()
		if err != nil {
			return fmt.Errorf("unable to get kube client: %w", err)
		}

		err = resources.DeleteResourcesLoop(ctx, kubeClient, detachResources)
		if err != nil {
			return fmt.Errorf("unable to delete resources: %w", err)
		}

		if err := commander.DeleteManagedByCommanderConfigMap(ctx, kubeClient); err != nil {
			return fmt.Errorf("unable to remove commander ConfigMap: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
