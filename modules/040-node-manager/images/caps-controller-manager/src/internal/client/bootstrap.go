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
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/scope"
)

const RequeueForStaticInstanceBootstrapping = 60 * time.Second

// Bootstrap runs the bootstrap script on StaticInstance.
func (c *Client) Bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	result, err := c.bootstrap(ctx, instanceScope)
	if err != nil {
		return result, err
	}

	if result.IsZero() {
		result.RequeueAfter = RequeueForStaticInstanceBootstrapping
	}

	return result, nil
}

func (c *Client) bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	phase := instanceScope.GetPhase()

	if phase != deckhousev1.StaticInstanceStatusCurrentStatusPhasePending &&
		phase != deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
		return ctrl.Result{}, errors.New("StaticInstance is not pending or bootstrapping")
	}

	result, err := c.bootstrapStaticInstance(ctx, instanceScope)
	if err != nil {
		return result, errors.Wrapf(err, "failed to bootstrap StaticInstance from '%s' phase", phase)
	}

	return result, nil
}

func (c *Client) bootstrapStaticInstance(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	bootstrapScript, err := getBootstrapScript(ctx, instanceScope)
	if err != nil {
		c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "BootstrapScriptFetchingFailed", "Bootstrap script unreachable")

		return ctrl.Result{}, errors.Wrap(err, "failed to get bootstrap script")
	}

	const operation = "bootstrap"

	innerLogger := getLogger(instanceScope, operation)

	if instanceScope.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhasePending {
		innerLogger.Info("set instance to bootstrapping status", "instance", instanceScope.InstanceName())
		result, err := c.setStaticInstancePhaseToBootstrapping(ctx, instanceScope)
		if err != nil {
			return result, err
		}
		if !result.IsZero() {
			return result, nil
		}
	}

	id := taskID(instanceScope.MachineScope.StaticMachine.Spec.ProviderID)
	done := c.bootstrapTaskManager.spawn(id, func() bool {
		logger := getLogger(instanceScope, operation)

		logger.Info("start new task", "id", id)
		sshCl, err := CreateSSHClient(instanceScope)
		if err != nil {
			logger.Error(err, "Failed to bootstrap StaticInstance: failed to create ssh client")
			return false
		}

		logger.Info("exec bootstrap command...")

		data, err := sshCl.ExecSSHCommandToString(instanceScope,
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name && echo '%s' | base64 -d | bash",
				instanceScope.MachineScope.StaticMachine.Spec.ProviderID, instanceScope.MachineScope.Machine.Name, base64.StdEncoding.EncodeToString(bootstrapScript)))
		if err != nil {
			logger.Info("bootstrap command failed", "error", err.Error())
			const alreadyBootstrappedMsg = "Probably instance already bootstrapped, because we got exit status 2"
			if strings.Contains(err.Error(), "Process exited with status 2") {
				logger.Info(alreadyBootstrappedMsg)
				return true
			}
			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					logger.Info(alreadyBootstrappedMsg)
					return true
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			logger.Error(err, "Failed to bootstrap StaticInstance: failed to exec ssh command")
			return false
		}

		logger.Info("bootstrap command executed successfully")

		return true
	})
	if done == nil || !*done {
		innerLogger.Info("Bootstrapping is not finished yet, waiting...")
		return ctrl.Result{}, nil
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "BootstrapScriptSucceeded", "Bootstrap script executed successfully")

	if instanceScope.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
		innerLogger.Info("set phase to Running")
		err := c.setStaticInstancePhaseToRunning(ctx, instanceScope)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (c *Client) setStaticInstancePhaseToBootstrapping(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	address := net.JoinHostPort(instanceScope.Instance.Spec.Address, strconv.Itoa(instanceScope.Credentials.Spec.SSHPort))

	delay := c.tcpCheckRateLimiter.When(address)

	const operation = "setStaticInstancePhaseToBootstrapping"

	listenSSHID := taskID(address)

	done := c.tcpCheckTaskManager.spawn(listenSSHID, func() bool {
		logger := getLogger(instanceScope, "tryToListenSSHServer")

		logger.Info("start new task", "id", listenSSHID)

		status := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckTcpConnection)
		logger.Info("Waiting for TCP connection for boostrap with timeout", "address", address, "timeout", delay.String())
		conn, err := net.DialTimeout("tcp", address, delay)
		if err != nil {
			logger.Error(err, "Failed to connect to instance by TCP", "address", address, "error", err.Error())
			if status == nil || status.Status != corev1.ConditionFalse || status.Reason != err.Error() {
				c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "StaticInstanceTcpFailed", err.Error())
				logger.Error(err, "Failed to check the StaticInstance address by establishing a tcp connection", "address", address)
				conditions.MarkFalse(instanceScope.Instance, infrav1.StaticInstanceCheckTcpConnection, err.Error(), clusterv1.ConditionSeverityError, "")
				err2 := instanceScope.Patch(ctx)
				if err2 != nil {
					logger.Error(err, "Failed to set StaticInstance: tcpCheck")
				}
			}
			return false
		}
		defer conn.Close()
		if status == nil || status.Status != corev1.ConditionTrue {
			conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceCheckTcpConnection)
			err := instanceScope.Patch(ctx)
			if err != nil {
				logger.Error(err, "Failed to set StaticInstance: tcpCheck")
			}
		}

		logger.Info("listen ssh connection finished successfully")

		return true
	})
	if done == nil {
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	if !*done {
		err := errors.New("Failed to connect via tcp")
		getLogger(instanceScope, operation).Error(err, "Failed to connect via tcp to StaticInstance address", "address", address)
		return ctrl.Result{}, err
	}

	c.tcpCheckRateLimiter.Forget(address)

	connectWIthCredsID := taskID(address)

	check := c.checkTaskManager.spawn(connectWIthCredsID, func() bool {
		logger := getLogger(instanceScope, "connectToSSHWIthCreds")

		logger.Info("start new task", "id", connectWIthCredsID)

		status := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckSshCondition)
		sshCl, err := CreateSSHClient(instanceScope)
		if err != nil {
			logger.Error(err, "Failed to set StaticInstance: Failed to connect via ssh")
		}

		logger.Info("exec ssh 'echo check_ssh' command...")

		data, err := sshCl.ExecSSHCommandToString(instanceScope, "echo check_ssh")
		if err != nil {
			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				isTimeout := strings.Contains(str, "Connection to ") && strings.Contains(str, " timed out")
				isPermissionDenied := strings.Contains(str, "Permission denied (publickey).")
				if isTimeout || isPermissionDenied {
					logger.Info("failed to connect via ssh", "isTimeout", fmt.Sprintf("%v", isTimeout), "isPermissionDenied", fmt.Sprintf("%v", isPermissionDenied))
					err := errors.New(str)
					if status == nil || status.Status != corev1.ConditionFalse || status.Reason != err.Error() {
						c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "StaticInstanceSshFailed", str)
						logger.Error(err, "StaticInstance: Failed to connect via ssh. Set condition")
						conditions.MarkFalse(instanceScope.Instance, infrav1.StaticInstanceCheckSshCondition, err.Error(), clusterv1.ConditionSeverityError, "")
						err2 := instanceScope.Patch(ctx)
						if err2 != nil {
							logger.Error(err, "Failed to set StaticInstance: Failed to connect via ssh. Cannot set condition")
						}
					}
				}
			}
			return false
		}
		if status == nil || status.Status != corev1.ConditionTrue {
			conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceCheckSshCondition)
			errPatch := instanceScope.Patch(ctx)
			if err != nil {
				logger.Error(errPatch, "Failed to set condition to StaticInstance that it was connected: Failed to connect via ssh")
			}
		}

		logger.Info("connected to ssh server with credentials finished successfully")

		return true
	})
	if check == nil {
		return ctrl.Result{RequeueAfter: delay}, nil
	}
	if !*check {
		err := errors.New("Failed to connect via ssh")
		getLogger(instanceScope, operation).Error(err, "Failed to connect via ssh to StaticInstance address", "address", address)
		return ctrl.Result{}, err
	}

	providerID := providerid.GenerateProviderID(instanceScope.Instance.Name)

	instanceScope.MachineScope.StaticMachine.Spec.ProviderID = providerID

	err := instanceScope.MachineScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to set StaticMachine provider id to '%s'", providerID)
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
		return ctrl.Result{}, errors.Wrap(err, "failed to patch StaticInstance MachineRef and Phase")
	}

	return ctrl.Result{}, nil
}

// setStaticInstancePhaseToRunning finishes the bootstrap process by waiting for bootstrapping Node to appear and patching StaticMachine and StaticInstance.
func (c *Client) setStaticInstancePhaseToRunning(ctx context.Context, instanceScope *scope.InstanceScope) error {
	node, err := getNodeByProviderID(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to get Node by provider id")
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "NodeBootstrappingSucceeded", "Node successfully bootstrapped")

	getLogger(instanceScope, "setPhaseToRunning").Info("Node successfully bootstrapped", "node", node.Name)

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

// getNodeByProviderID returns the Node with the provider id from the StaticMachine's spec.
func getNodeByProviderID(ctx context.Context, instanceScope *scope.InstanceScope) (*corev1.Node, error) {
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

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return node, nil
		}
	}

	return nil, errors.Errorf("Node '%s' is not ready", node.Name)
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

func mapAddresses(addresses []corev1.NodeAddress) clusterv1.MachineAddresses {
	var machineAddresses clusterv1.MachineAddresses

	for _, address := range addresses {
		machineAddresses = append(machineAddresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineAddressType(address.Type),
			Address: address.Address,
		})
	}

	return machineAddresses
}
