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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
)

func (c *Client) AdoptStaticInstance(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	machine *clusterv1.Machine,
) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("adopting node for StaticInstance", "annotation", deckhousev1.SkipBootstrapPhaseAnnotation)

	credentials := &deckhousev1.SSHCredentials{}
	if err := c.client.Get(ctx, client.ObjectKey{Name: staticInstance.Spec.CredentialsRef.Name}, credentials); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to load SSHCredentials: %w", err)
	}

	sshLegacyMode := true
	if len(credentials.Spec.PrivateSSHKey) == 0 {
		sshLegacyMode = false
	}

	if staticMachine.Spec.ProviderID == "" {
		providerID := providerid.GenerateProviderID(staticInstance.Name)
		staticMachine.Spec.ProviderID = providerID
	}

	staticInstance.Status.MachineRef = &corev1.ObjectReference{
		APIVersion: staticMachine.APIVersion,
		Kind:       staticMachine.Kind,
		Namespace:  staticMachine.Namespace,
		Name:       staticMachine.Name,
		UID:        staticMachine.UID,
	}

	if err := c.adoptStaticInstance(ctx, staticInstance, staticMachine, machine, credentials.Spec, sshLegacyMode); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to adopt static instance: %w", err)
	}

	delete(staticInstance.Annotations, deckhousev1.SkipBootstrapPhaseAnnotation)

	if err := c.setStaticInstancePhaseToRunning(ctx, staticInstance, staticMachine); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set StaticInstance phase to Running: %w", err)
	}

	return ctrl.Result{}, nil
}

func (c *Client) adoptStaticInstance(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	machine *clusterv1.Machine,
	credentials deckhousev1.SSHCredentialsSpec,
	sshLegacyMode bool) error {
	type taskDataStr struct {
		address       string
		credentials   deckhousev1.SSHCredentialsSpec
		sshLegacyMode bool

		providerID  string
		machineName string
	}

	taskFunc := func(tCtx context.Context, tAny any) error {
		tLogger := ctrl.LoggerFrom(tCtx)
		t, ok := tAny.(taskDataStr)
		if !ok {
			return errors.New("invalid task data")
		}

		var sshCl ssh.SSH
		var err error
		if t.sshLegacyMode {
			tLogger.Info("using clissh")
			sshCl = clissh.CreateSSHClient(t.address, t.credentials)
		} else {
			tLogger.Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(t.address, t.credentials)
		}

		if err != nil {
			tLogger.Error(err, "failed to create ssh client")
			return fmt.Errorf("failed to create ssh client: %w", err)
		}

		data, err := sshCl.ExecSSHCommandToString(
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name",
				t.providerID, t.machineName))
		if err != nil {
			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					return nil
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			tLogger.Error(err, "failed to exec ssh command")
			return fmt.Errorf("failed to exec ssh command: %w", err)
		}
		return nil
	}

	taskData := taskDataStr{
		address:       staticInstance.Spec.Address,
		credentials:   credentials,
		sshLegacyMode: sshLegacyMode,
		providerID:    machine.Spec.ProviderID,
		machineName:   machine.Name,
	}

	taskCtx := ctrl.LoggerInto(c.taskManagerCtx, ctrl.LoggerFrom(ctx))
	err, finished := c.taskManager.Spawn(taskCtx, string(staticMachine.Spec.ProviderID), "adopt", taskData, taskFunc)
	if err != nil {
		return err
	}

	logger := ctrl.LoggerFrom(ctx)
	if finished {
		logger.Info("Adoption script executed successfully")
		c.recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "AdoptionScriptSucceeded", "Adoption script executed successfully")
		return nil
	}

	logger.Info("Adopting is not finished yet, waiting...")
	return nil
}
