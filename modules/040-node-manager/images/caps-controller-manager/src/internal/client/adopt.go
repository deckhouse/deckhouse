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
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
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
	done := c.adoptTaskManager.spawn(taskID(instanceScope.MachineScope.StaticMachine.Spec.ProviderID), func() bool {
		var sshCl ssh.SSH
		var err error
		if instanceScope.SSHLegacyMode {
			instanceScope.Logger.Info("using clissh")
			sshCl, err = clissh.CreateSSHClient(instanceScope)
		} else {
			instanceScope.Logger.Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(instanceScope)
		}

		if err != nil {
			instanceScope.Logger.Error(err, "Failed to adopt StaticInstance: failed to create ssh client")
			return false
		}
		data, err := sshCl.ExecSSHCommandToString(instanceScope,
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name",
				instanceScope.MachineScope.StaticMachine.Spec.ProviderID, instanceScope.MachineScope.Machine.Name))
		if err != nil {
			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					return true
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			instanceScope.Logger.Error(err, "Failed to adopt StaticInstance: failed to exec ssh command")
			return false
		}

		return true
	})
	if done == nil || !*done {
		instanceScope.Logger.Info("Adopting is not finished yet, waiting...")
		return false, nil
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "AdoptionScriptSucceeded", "Adoption script executed successfully")

	return true, nil
}
