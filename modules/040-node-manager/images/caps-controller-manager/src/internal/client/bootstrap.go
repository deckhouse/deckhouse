/*
Copyright 2026 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
)

const (
	RequeueForStaticInstanceBootstrapping = 1 * time.Minute
)

// Bootstrap runs the bootstrap script on StaticInstance.
func (c *Client) Bootstrap(ctx context.Context, staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine, machine *clusterv1.Machine) (ctrl.Result, error) {
	if phase := staticInstance.GetPhase(); phase != deckhousev1.StaticInstanceStatusCurrentStatusPhasePending &&
		phase != deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
		return ctrl.Result{}, errors.New("StaticInstance is not pending or bootstrapping")
	}

	result, err := c.bootstrapStaticInstance(ctx, staticInstance, staticMachine, machine)
	if err != nil {
		return result, fmt.Errorf("failed to bootstrap StaticInstance from '%s' phase: %w", staticInstance.GetPhase(), err)
	}

	return result, nil
}

func (c *Client) bootstrapStaticInstance(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	machine *clusterv1.Machine) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &deckhousev1.SSHCredentials{}
	if err := c.client.Get(ctx, client.ObjectKey{Name: staticInstance.Spec.CredentialsRef.Name}, credentials); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to load SSHCredentials: %w", err)
	}

	sshLegacyMode := true //nolint:staticcheck
	if len(credentials.Spec.PrivateSSHKey) == 0 {
		sshLegacyMode = false
	}

	if staticInstance.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhasePending || staticMachine.Spec.ProviderID == "" {
		return c.setStaticInstancePhaseToBootstrapping(ctx, staticInstance, staticMachine, credentials.Spec, sshLegacyMode)
	}

	bootstrapScript, err := c.getBootstrapScript(ctx, staticMachine, machine)
	if err != nil {
		c.recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "BootstrapScriptFetchingFailed", "Bootstrap script unreachable")
		return ctrl.Result{}, fmt.Errorf("failed to get bootstrap script: %w", err)
	}

	type taskDataStr struct {
		host          string
		credentials   deckhousev1.SSHCredentialsSpec
		sshLegacyMode bool

		providerID      string
		machineName     string
		bootstrapScript []byte
	}

	taskData := taskDataStr{
		host:            staticInstance.Spec.Address,
		credentials:     credentials.Spec,
		sshLegacyMode:   sshLegacyMode,
		providerID:      string(staticMachine.Spec.ProviderID),
		machineName:     machine.Name,
		bootstrapScript: bootstrapScript,
	}

	taskFunc := func(tCtx context.Context, tAny any) error {
		tLogger := ctrl.LoggerFrom(tCtx)
		t, ok := tAny.(taskDataStr)
		if !ok {
			return errors.New("invalid task data")
		}

		var sshCl ssh.SSH
		var tErr error
		if t.sshLegacyMode {
			tLogger.Info("using clissh")
			sshCl = clissh.CreateSSHClient(t.host, t.credentials)
		} else {
			tLogger.Info("using gossh")
			sshCl, tErr = gossh.CreateSSHClient(t.host, t.credentials)
		}
		if tErr != nil {
			tLogger.Error(err, "failed to create ssh client")
			return fmt.Errorf("failed to create ssh client: %w", tErr)
		}
		tLogger.Info("bootstrapping node")
		tRes, tErr := sshCl.ExecSSHCommandToString(
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name && echo '%s' | base64 -d | bash",
				t.providerID, t.machineName, base64.StdEncoding.EncodeToString(t.bootstrapScript)))
		if tErr != nil {
			if strings.Contains(tErr.Error(), "Process exited with status 2") {
				return nil
			}
			scanner := bufio.NewScanner(strings.NewReader(tRes))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					return nil
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			tLogger.Error(tErr, "failed to exec ssh command")
			return fmt.Errorf("failed to exec ssh command: %w", tErr)
		}
		return nil
	}

	logger = logger.WithValues("taskID", string(staticMachine.Spec.ProviderID))
	logger.Info("Running bootstrap task")
	err, finished := c.taskManager.Spawn(c.taskManagerCtx, string(staticMachine.Spec.ProviderID), "bootstrap", taskData, taskFunc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to bootstrap StaticInstance: %w", err)
	}

	if finished {
		logger.Info("Bootstrap script executed successfully")
		c.recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "BootstrapScriptSucceeded", "Bootstrap script executed successfully")
		if staticInstance.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
			if err = c.setStaticInstancePhaseToRunning(ctx, staticInstance, staticMachine); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	logger.Info("Bootstrapping is not finished yet, requeuing")
	return ctrl.Result{RequeueAfter: RequeueForStaticInstanceBootstrapping}, nil
}

//nolint:nonamedreturns
func (c *Client) setStaticInstancePhaseToBootstrapping(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	credentials deckhousev1.SSHCredentialsSpec,
	sshLegacyMode bool) (res ctrl.Result, resErr error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("staticMachineUID", staticMachine.UID, "staticInstanceAddress", staticInstance.Spec.Address)

	if err := c.reserveStaticInstance(staticInstance, staticMachine); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reserve StaticInstance: %w", err)
	}

	defer func() {
		if resErr != nil {
			logger.Info("Releasing StaticInstance reservation due to error", "error", resErr.Error())
			c.releaseStaticInstance(staticInstance, staticMachine)
		}
	}()

	address := net.JoinHostPort(staticInstance.Spec.Address, strconv.Itoa(credentials.SSHPort))
	delay := c.tcpCheckRateLimiter.When(address)
	defer c.tcpCheckRateLimiter.Forget(address)

	tcpCondition := conditions.Get(staticInstance, infrav1.StaticInstanceCheckTCPConnection)
	if tcpCondition == nil || tcpCondition.Status != metav1.ConditionTrue {
		type taskDataStr struct {
			address string
			delay   time.Duration
		}

		taskData := taskDataStr{
			address: address,
			delay:   delay,
		}

		taskFunc := func(tCtx context.Context, data any) error {
			start := time.Now()
			tLogger := ctrl.LoggerFrom(tCtx)

			t, ok := data.(taskDataStr)
			if !ok {
				return errors.New("invalid task data")
			}

			tLogger.Info("Checking TCP connection", "address", t.address, "timeout", t.delay.String())
			conn, tErr := net.DialTimeout("tcp", t.address, t.delay)
			if tErr != nil {
				tLogger.Error(tErr, "Failed to connect to instance by TCP", "address", t.address)
				return fmt.Errorf("Failed to check the StaticInstance address by establishing a tcp connection: %w", tErr)
			}

			defer conn.Close()

			tLogger.Info("TCP connection check completed successfully", "elapsed", time.Since(start))
			return nil
		}

		logger.Info("Running TCP check task", "taskID", address)
		err, finished := c.taskManager.Spawn(c.taskManagerCtx, address, "tcp-check", taskData, taskFunc)
		if err != nil {
			c.recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceTcpFailed", err.Error())
			logger.Error(err, "Failed to check the StaticInstance address by establishing a tcp connection")

			conditions.Set(staticInstance, metav1.Condition{
				Type:               infrav1.StaticInstanceCheckTCPConnection,
				Status:             metav1.ConditionFalse,
				Reason:             infrav1.StaticInstanceCheckFailedReason,
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})

			return ctrl.Result{RequeueAfter: delay}, nil
		}

		if !finished {
			logger.Info("TCP check still running, requeueing", "requeueAfter", delay)
			return ctrl.Result{RequeueAfter: delay}, nil
		}

		logger.Info("TCP connection check passed")
		conditions.Set(staticInstance, metav1.Condition{
			Type:               infrav1.StaticInstanceCheckTCPConnection,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.StaticInstanceCheckPassedReason,
			Message:            "TCP connection check passed",
			LastTransitionTime: metav1.Now(),
		})
	}

	if sshCondition := conditions.Get(staticInstance, infrav1.StaticInstanceCheckSSHCondition); sshCondition == nil ||
		sshCondition.Status != metav1.ConditionTrue {
		type taskDataStr struct {
			host          string
			address       string
			credentials   deckhousev1.SSHCredentialsSpec
			sshLegacyMode bool
		}

		taskData := taskDataStr{
			host:          staticInstance.Spec.Address,
			address:       address,
			credentials:   credentials,
			sshLegacyMode: sshLegacyMode,
		}

		taskFunc := func(tCtx context.Context, data any) error {
			start := time.Now()
			tLogger := ctrl.LoggerFrom(tCtx)

			t, ok := data.(taskDataStr)
			if !ok {
				return errors.New("invalid task data")
			}

			var sshCl ssh.SSH
			var tErr error
			if t.sshLegacyMode {
				tLogger.Info("using clissh")
				sshCl = clissh.CreateSSHClient(t.host, t.credentials)
			} else {
				tLogger.Info("using gossh")
				sshCl, tErr = gossh.CreateSSHClient(t.host, t.credentials)
			}
			if tErr != nil {
				tLogger.Error(tErr, "Failed to connect via ssh")
				return fmt.Errorf("failed to connect via ssh with address %s: %w", t.address, tErr)
			}
			tRes, tErr := sshCl.ExecSSHCommandToString("echo check_ssh")
			if tErr != nil {
				scanner := bufio.NewScanner(strings.NewReader(tRes))
				for scanner.Scan() {
					str := scanner.Text()
					if (strings.Contains(str, "Connection to ") && strings.Contains(str, " timed out")) ||
						strings.Contains(str, "Permission denied (publickey).") ||
						strings.Contains(str, "Could not resolve hostname") {
						return errors.New(str)
					}
				}
				return tErr
			}

			tLogger.Info("SSH connectivity check completed", "elapsed", time.Since(start))
			return nil
		}

		logger.Info("Running SSH check task", "taskID", address)
		err, finished := c.taskManager.Spawn(c.taskManagerCtx, address, "ssh-check", taskData, taskFunc)
		if err != nil {
			logger.Error(err, "Failed to connect via ssh to StaticInstance address")
			c.recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceSshFailed", err.Error())

			conditions.Set(staticInstance, metav1.Condition{
				Type:               infrav1.StaticInstanceCheckSSHCondition,
				Status:             metav1.ConditionFalse,
				Reason:             infrav1.StaticInstanceCheckFailedReason,
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})

			return ctrl.Result{}, err
		}

		if !finished {
			logger.Info("SSH check still running, requeueing", "requeueAfter", delay)
			return ctrl.Result{RequeueAfter: delay}, nil
		}

		logger.Info("SSH connectivity check passed")
		conditions.Set(staticInstance, metav1.Condition{
			Type:               infrav1.StaticInstanceCheckSSHCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.StaticInstanceCheckPassedReason,
			Message:            "SSH connectivity check passed",
			LastTransitionTime: metav1.Now(),
		})
	}

	staticMachine.Spec.ProviderID = providerid.GenerateProviderID(staticInstance.Name)
	return ctrl.Result{RequeueAfter: RequeueForStaticInstanceBootstrapping}, nil
}

func (c *Client) reserveStaticInstance(staticInstance *deckhousev1.StaticInstance, staticMachine *infrav1.StaticMachine) error {
	currentRef := staticInstance.Status.MachineRef

	if currentRef != nil && currentRef.UID == staticMachine.UID {
		if staticInstance.GetPhase() != deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
			staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)
		}

		return nil
	}

	if currentRef != nil && currentRef.UID != staticMachine.UID {
		return errors.Errorf("StaticInstance already reserved for another StaticMachine: %s", currentRef.Name)
	}

	staticInstance.Status.MachineRef = &corev1.ObjectReference{
		APIVersion: staticMachine.APIVersion,
		Kind:       staticMachine.Kind,
		Namespace:  staticMachine.Namespace,
		Name:       staticMachine.Name,
		UID:        staticMachine.UID,
	}

	staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)
	return nil
}

func (c *Client) releaseStaticInstance(staticInstance *deckhousev1.StaticInstance, staticMachine *infrav1.StaticMachine) {
	if staticInstance.Status.MachineRef == nil || staticInstance.Status.MachineRef.UID != staticMachine.UID {
		return
	}

	staticInstance.ToPending()
}

// setStaticInstancePhaseToRunning finishes the bootstrap process by waiting for bootstrapping Node to appear and patching StaticMachine and StaticInstance.
func (c *Client) setStaticInstancePhaseToRunning(ctx context.Context, staticInstance *deckhousev1.StaticInstance, staticMachine *infrav1.StaticMachine) error {
	logger := ctrl.LoggerFrom(ctx)

	node, err := c.getNodeByProviderID(ctx, staticMachine)
	if err != nil {
		return err
	}

	c.recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "NodeBootstrappingSucceeded", "Node successfully bootstrapped")
	logger.Info("Node successfully bootstrapped", "node", node.Name)

	staticMachine.Status.Addresses = mapAddresses(node.Status.Addresses)

	conditions.Set(staticInstance, metav1.Condition{
		Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             infrav1.StaticInstanceCheckPassedReason,
		Message:            "StaticInstance is ready",
		LastTransitionTime: metav1.Now(),
	})

	staticInstance.Status.NodeRef = &corev1.ObjectReference{
		APIVersion: node.APIVersion,
		Kind:       node.Kind,
		Name:       node.Name,
		UID:        node.UID,
	}

	conditions.Set(staticInstance, metav1.Condition{
		Type:               infrav1.StaticInstanceBootstrapSucceededCondition,
		Message:            "StaticInstance is bootstrapped",
		Reason:             infrav1.StaticInstanceBootstrapSucceededCondition,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})

	staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning)
	return nil
}

// getNodeByProviderID returns the Node with the provider id from the StaticMachine's spec.
func (c *Client) getNodeByProviderID(ctx context.Context, staticMachine *infrav1.StaticMachine) (*corev1.Node, error) {
	nodes := &corev1.NodeList{}
	nodeSelector := fields.OneTermEqualSelector("spec.providerID", string(staticMachine.Spec.ProviderID))

	if err := c.client.List(
		ctx,
		nodes,
		client.MatchingFieldsSelector{Selector: nodeSelector},
	); err != nil {
		return nil, fmt.Errorf("failed to find Node by provider id '%s': %w", staticMachine.Spec.ProviderID, err)
	}

	if len(nodes.Items) == 0 {
		return nil, errors.Errorf("Node with provider id '%s' not found", staticMachine.Spec.ProviderID)
	}

	if len(nodes.Items) > 1 {
		return nil, errors.Errorf("found more than one Node with provider id '%s'", staticMachine.Spec.ProviderID)
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
func (c *Client) getBootstrapScript(ctx context.Context, staticMachine *infrav1.StaticMachine, machine *clusterv1.Machine) ([]byte, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: staticMachine.Namespace,
		Name:      *machine.Spec.Bootstrap.DataSecretName,
	}

	if err := c.client.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("failed to retrieve bootstrap data secret for StaticMachine '%s/%s': %w",
			staticMachine.Namespace,
			staticMachine.Name,
			err,
		)
	}

	bootstrapScript, ok := secret.Data["bootstrap.sh"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret 'bootstrap.sh' key is missing")
	}

	return bootstrapScript, nil
}

func mapAddresses(addresses []corev1.NodeAddress) clusterv1.MachineAddresses {
	machineAddresses := make(clusterv1.MachineAddresses, 0, len(addresses))

	for _, address := range addresses {
		machineAddresses = append(machineAddresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineAddressType(address.Type),
			Address: address.Address,
		})
	}

	return machineAddresses
}
