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

package client

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/scope"
)

func (c *Client) AdoptStaticInstance(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	instanceScope.Logger.Info(
		fmt.Sprintf("adopting node for StaticInstance with '%s' annotation", deckhousev1.SkipBootstrapPhaseAnnotation),
	)

	if instanceScope.MachineScope.StaticMachine.Spec.ProviderID == "" {
		providerID := providerid.GenerateProviderID(instanceScope.Instance.Name)

		instanceScope.MachineScope.StaticMachine.Spec.ProviderID = providerID

		err := instanceScope.MachineScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set StaticMachine provider id to '%s'", providerID)
		}
	}

	instanceScope.Instance.Status.MachineRef = &corev1.ObjectReference{
		APIVersion: instanceScope.MachineScope.StaticMachine.APIVersion,
		Kind:       instanceScope.MachineScope.StaticMachine.Kind,
		Namespace:  instanceScope.MachineScope.StaticMachine.Namespace,
		Name:       instanceScope.MachineScope.StaticMachine.Name,
		UID:        instanceScope.MachineScope.StaticMachine.UID,
	}

	err := instanceScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch StaticInstance MachineRef")
	}

	ok, err := c.adoptStaticInstance(instanceScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to adopt StaticInstance")
	}
	if !ok {
		return ctrl.Result{}, nil
	}

	delete(instanceScope.Instance.Annotations, deckhousev1.SkipBootstrapPhaseAnnotation)

	err = c.setStaticInstancePhaseToRunning(ctx, instanceScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to set StaticInstance phase to Running")
	}

	return ctrl.Result{}, nil
}

func (c *Client) adoptStaticInstance(instanceScope *scope.InstanceScope) (bool, error) {
	const operation = "adoptStaticInstance"

	id := taskID(instanceScope.MachineScope.StaticMachine.Spec.ProviderID)

	done := c.adoptTaskManager.spawn(id, func() bool {
		logger := getLogger(instanceScope, operation)

		logger.Info("start new task", "id", id)

		sshCl, err := CreateSSHClient(instanceScope)
		if err != nil {
			logger.Error(err, "Failed to adopt StaticInstance: failed to create ssh client")
			return false
		}

		logger.Info("exec adopt ssh command...")

		data, err := sshCl.ExecSSHCommandToString(instanceScope,
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name",
				instanceScope.MachineScope.StaticMachine.Spec.ProviderID, instanceScope.MachineScope.Machine.Name))
		if err != nil {
			logger.Info("failed to exec adopt ssh command", "err", err.Error())

			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					logger.Info("Probably instance already adopted because we get exit status 2")
					return true
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			logger.Error(err, "Failed to adopt StaticInstance: failed to exec ssh command")
			return false
		}

		logger.Info("Adopt command executed successfully")

		return true
	})
	if done == nil || !*done {
		getLogger(instanceScope, operation).Info("Adopting is not finished yet, waiting...")
		return false, nil
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "AdoptionScriptSucceeded", "Adoption script executed successfully")

	return true, nil
}
