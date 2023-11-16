/*
Copyright 2023 Flant JSC

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
	"context"
	"encoding/base64"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha1"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/scope"
	"caps-controller-manager/internal/ssh"
)

// Bootstrap runs the bootstrap script on StaticInstance.
func (c *Client) Bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) error {
	switch instanceScope.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhasePending:
		err := c.bootstrapFromPendingPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to bootstrap StaticInstance from pending phase")
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping:
		err := c.bootstrapFromBootstrappingPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to bootstrap StaticInstance from bootstrapping phase")
		}
	default:
		return errors.New("StaticInstance is not pending or bootstrapping")
	}

	return nil
}

func (c *Client) bootstrapFromPendingPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	providerID := providerid.GenerateProviderID()

	instanceScope.MachineScope.StaticMachine.Spec.ProviderID = providerID

	err := instanceScope.MachineScope.Patch(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to set StaticMachine provider id to '%s'", providerID)
	}

	instanceScope.Instance.Status.MachineRef = &corev1.ObjectReference{
		APIVersion: instanceScope.MachineScope.StaticMachine.APIVersion,
		Kind:       instanceScope.MachineScope.StaticMachine.Kind,
		Namespace:  instanceScope.MachineScope.StaticMachine.Namespace,
		Name:       instanceScope.MachineScope.StaticMachine.Name,
		UID:        instanceScope.MachineScope.StaticMachine.UID,
	}

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	err = instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance MachineRef and Phase")
	}

	_, err = c.bootstrap(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to bootstrap")
	}

	return nil
}

// bootstrapFromBootstrappingPhase finishes the bootstrap process by waiting for bootstrapping Node to appear and patching StaticMachine and StaticInstance.
func (c *Client) bootstrapFromBootstrappingPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	done, err := c.bootstrap(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to bootstrap")
	}
	if !done {
		return nil
	}

	node, err := waitForNode(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to wait for Node to appear")
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "NodeBootstrappingSucceeded", "Node successfully bootstrapped")

	instanceScope.Logger.Info("Node successfully bootstrapped", "node", node.Name)

	instanceScope.MachineScope.StaticMachine.Status.Addresses = mapAddresses(node.Status.Addresses)

	conditions.MarkTrue(instanceScope.MachineScope.StaticMachine, infrav1.StaticMachineStaticInstanceReadyCondition)

	instanceScope.Instance.Status.NodeRef = &corev1.ObjectReference{
		APIVersion: node.APIVersion,
		Kind:       node.Kind,
		Name:       node.Name,
		UID:        node.UID,
	}

	conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition)

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning)

	err = instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance NodeRef and Phase")
	}

	err = instanceScope.MachineScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticMachine with Node provider id and addresses")
	}

	return nil
}

func (c *Client) bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) (bool, error) {
	bootstrapScript, err := getBootstrapScript(ctx, instanceScope)
	if err != nil {
		c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "BootstrapScriptFetchingFailed", "Bootstrap script unreachable")

		return false, errors.Wrap(err, "failed to get bootstrap script")
	}

	done := c.spawn(instanceScope.MachineScope.StaticMachine.Spec.ProviderID, func() bool {
		err := ssh.ExecSSHCommand(instanceScope, fmt.Sprintf("mkdir /var/lib/bashible || exit 0 && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name && echo '%s' | base64 -d | bash", instanceScope.MachineScope.StaticMachine.Spec.ProviderID, instanceScope.MachineScope.Machine.Name, base64.StdEncoding.EncodeToString(bootstrapScript)), nil)
		if err != nil {
			// If Node reboots, the ssh connection will close, and we will get an error.
			instanceScope.Logger.Error(err, "Failed to bootstrap StaticInstance: failed to exec ssh command")

			return false
		}

		return true
	})
	if done {
		c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "BootstrapScriptSucceeded", "Bootstrap script executed successfully")
	} else {
		instanceScope.Logger.Info("Bootstrapping is not finished yet, waiting...")
	}

	return done, nil
}

// waitForNode waits for the node to appear and checks that it has 'node.deckhouse.io/configuration-checksum' annotation.
func waitForNode(ctx context.Context, instanceScope *scope.InstanceScope) (*corev1.Node, error) {
	nodes := &corev1.NodeList{}
	nodeSelector := fields.OneTermEqualSelector("spec.providerID", string(instanceScope.MachineScope.StaticMachine.Spec.ProviderID))

	err := instanceScope.Client.List(
		ctx,
		nodes,
		client.MatchingFieldsSelector{Selector: nodeSelector},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Node by provider id '%s'", instanceScope.MachineScope.StaticMachine.Spec.ProviderID)
	}

	if len(nodes.Items) == 0 {
		return nil, errors.Errorf("Node with provider id '%s' not found", instanceScope.MachineScope.StaticMachine.Spec.ProviderID)
	}

	if len(nodes.Items) > 1 {
		return nil, errors.Errorf("found more than one Node with provider id '%s'", instanceScope.MachineScope.StaticMachine.Spec.ProviderID)
	}

	node := &nodes.Items[0]

	if node.Annotations["node.deckhouse.io/configuration-checksum"] == "" {
		return nil, errors.Errorf("Node '%s' doesn't have 'node.deckhouse.io/configuration-checksum' annotation", node.Name)
	}

	return node, nil
}

// getBootstrapScript returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func getBootstrapScript(ctx context.Context, instanceScope *scope.InstanceScope) ([]byte, error) {
	if instanceScope.MachineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: instanceScope.MachineScope.StaticMachine.Namespace,
		Name:      *instanceScope.MachineScope.Machine.Spec.Bootstrap.DataSecretName,
	}

	err := instanceScope.Client.Get(ctx, key, secret)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to retrieve bootstrap data secret for StaticMachine '%s/%s'",
			instanceScope.MachineScope.StaticMachine.Namespace,
			instanceScope.MachineScope.StaticMachine.Name,
		)
	}

	bootstrapScript, ok := secret.Data["bootstrap.sh"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret 'bootstrap.sh' key is missing")
	}

	return bootstrapScript, nil
}

func mapAddresses(addresses []corev1.NodeAddress) v1beta1.MachineAddresses {
	var machineAddresses v1beta1.MachineAddresses

	for _, address := range addresses {
		machineAddresses = append(machineAddresses, v1beta1.MachineAddress{
			Type:    v1beta1.MachineAddressType(address.Type),
			Address: address.Address,
		})
	}

	return machineAddresses
}
