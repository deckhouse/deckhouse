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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"caps-controller-manager/internal/scope"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
)

const (
	RequeueForStaticInstanceBootstrapping = 60 * time.Second
	// RequeueForCheckInProgress is how often a still running TCP/SSH check is polled.
	RequeueForCheckInProgress = 5 * time.Second
	// TCPCheckDialTimeout bounds a single TCP connectivity probe. It must not be derived
	// from the rate limiter delay: the first attempt would then get the base backoff
	// (a fraction of a second) as its dial timeout and fail on any real network.
	TCPCheckDialTimeout = 5 * time.Second
)

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

	if instanceScope.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhasePending ||
		instanceScope.MachineScope.StaticMachine.Spec.ProviderID == "" {
		result, err := c.setStaticInstancePhaseToBootstrapping(ctx, instanceScope)
		if err != nil {
			return result, err
		}
		if !result.IsZero() {
			return result, nil
		}
	}

	// The task outlives the reconcile that spawned it, so it must not inherit its
	// cancellation. The ssh layer applies its own connect and command timeouts.
	bootstrapCtx := context.WithoutCancel(ctx)

	// The StaticMachine UID, not the provider id: the latter is assigned mid-bootstrap, so a
	// task keyed by it would be looked up under a different key than the one it was spawned
	// with, and every machine still waiting for its id would share the empty key.
	done := c.bootstrapTaskManager.spawn(taskID(instanceScope.MachineScope.StaticMachine.UID), func() bool {
		var sshCl ssh.SSH
		var err error
		if instanceScope.SSHLegacyMode {
			instanceScope.Logger.V(1).Info("using clissh")
			sshCl, err = clissh.CreateSSHClient(instanceScope)
		} else {
			instanceScope.Logger.V(1).Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(instanceScope)
		}
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to bootstrap StaticInstance: failed to create ssh client")
			return false
		}
		data, err := sshCl.ExecSSHCommandToString(bootstrapCtx, instanceScope,
			fmt.Sprintf("mkdir -p /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' > /var/lib/bashible/machine-name && echo '%s' | base64 -d | bash",
				instanceScope.MachineScope.StaticMachine.Spec.ProviderID, instanceScope.MachineScope.Machine.Name, base64.StdEncoding.EncodeToString(bootstrapScript)))
		if err != nil {
			if strings.Contains(err.Error(), "Process exited with status 2") {
				return true
			}
			scanner := bufio.NewScanner(strings.NewReader(data))
			for scanner.Scan() {
				str := scanner.Text()
				if strings.Contains(str, "debug1: Exit status 2") {
					return true
				}
			}
			// If Node reboots, the ssh connection will close, and we will get an error.
			instanceScope.Logger.Error(err, "Failed to bootstrap StaticInstance: failed to exec ssh command")
			return false
		}

		return true
	})
	if done == nil || !*done {
		instanceScope.Logger.V(1).Info("Bootstrapping is not finished yet, waiting...")
		return ctrl.Result{}, nil
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "BootstrapScriptSucceeded", "Bootstrap script executed successfully")

	if instanceScope.GetPhase() == deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
		err := c.setStaticInstancePhaseToRunning(ctx, instanceScope)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (c *Client) setStaticInstancePhaseToBootstrapping(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	instanceScope.Logger.Info("Starting reservation process",
		"instance", instanceScope.Instance.Name,
		"machine", instanceScope.MachineScope.StaticMachine.Name,
		"machineUID", instanceScope.MachineScope.StaticMachine.UID,
		"address", instanceScope.Instance.Spec.Address,
	)

	// The reservation is deliberately kept for the whole bootstrap window: a failed check is
	// retried with the address backoff rather than released, so that a Pending write and the
	// watch event it produces cannot re-enqueue this StaticMachine.
	//
	// How the instance gets back to the pool depends on whether the host was ever touched,
	// which is exactly what an empty ProviderID tells: it is assigned at the end of this
	// function, after both checks pass, and the bootstrap script only runs for a non-empty one.
	//
	// Empty ProviderID - nothing ran on the host, so the bootstrap timeout in
	// reconcileStaticInstancePhase releases the instance itself, and the delete flow skips the
	// remote cleanup instead of rebooting a host caps never touched.
	//
	// Non-empty ProviderID - the host may be half-bootstrapped and must not be handed to
	// another StaticMachine as is. The reservation is held, the timeout only marks the
	// StaticMachine with CreateError, and the instance comes back through the
	// MachineHealthCheck the node-controller creates for static clusters
	// (nodeStartupTimeoutSeconds: 1200): remediation deletes the Machine, cleanup runs, and the
	// cleanup timeout moves the instance to Pending.
	if err := c.reserveStaticInstance(ctx, instanceScope); err != nil {
		instanceScope.Logger.Error(err, "Failed to reserve StaticInstance",
			"instance", instanceScope.Instance.Name,
			"machine", instanceScope.MachineScope.StaticMachine.Name,
		)
		return ctrl.Result{}, err
	}

	instanceScope.Logger.Info("StaticInstance successfully reserved",
		"instance", instanceScope.Instance.Name,
		"machine", instanceScope.MachineScope.StaticMachine.Name,
		"machineUID", instanceScope.MachineScope.StaticMachine.UID,
	)

	address := net.JoinHostPort(instanceScope.Instance.Spec.Address, strconv.Itoa(instanceScope.Credentials.Spec.SSHPort))

	tcpCondition := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckTCPConnection)
	if tcpCondition == nil || tcpCondition.Status != metav1.ConditionTrue {
		tcpTaskID := address
		instanceScope.Logger.V(1).Info("Scheduling TCP check",
			"address", address,
			"timeout", TCPCheckDialTimeout,
			"taskID", tcpTaskID,
			"machine", instanceScope.MachineScope.StaticMachine.Name,
		)
		done := c.tcpCheckTaskManager.spawn(taskID(tcpTaskID), func() bool {
			start := time.Now()
			status := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckTCPConnection)
			instanceScope.Logger.V(1).Info("Waiting for TCP connection for boostrap with timeout", "address", address, "timeout", TCPCheckDialTimeout.String())
			conn, err := net.DialTimeout("tcp", address, TCPCheckDialTimeout)
			if err != nil {
				instanceScope.Logger.Error(err, "Failed to connect to instance by TCP", "address", address, "error", err.Error())
				if status == nil || status.Status != metav1.ConditionFalse || status.Reason != err.Error() {
					c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "StaticInstanceTcpFailed", err.Error())

					instanceScope.Logger.Error(err, "Failed to check the StaticInstance address by establishing a tcp connection", "address", address)

					conditions.Set(instanceScope.Instance, metav1.Condition{
						Type:               infrav1.StaticInstanceCheckTCPConnection,
						Status:             metav1.ConditionFalse,
						Reason:             err.Error(),
						Message:            err.Error(),
						LastTransitionTime: metav1.Now(),
					})

					err2 := instanceScope.Patch(ctx)
					if err2 != nil {
						instanceScope.Logger.Error(err, "Failed to set StaticInstance: tcpCheck")
					}
				}
				return false
			}

			defer conn.Close()

			if status == nil || status.Status != metav1.ConditionTrue {
				conditions.Set(instanceScope.Instance, metav1.Condition{
					Type:               infrav1.StaticInstanceCheckTCPConnection,
					Status:             metav1.ConditionTrue,
					Reason:             infrav1.StaticInstanceCheckPassedReason,
					Message:            "TCP connection check passed",
					LastTransitionTime: metav1.Now(),
				})

				err := instanceScope.Patch(ctx)
				if err != nil {
					instanceScope.Logger.Error(err, "Failed to set StaticInstance: tcpCheck")
				}
			}
			instanceScope.Logger.Info("TCP connection check completed successfully",
				"address", address,
				"machine", instanceScope.MachineScope.StaticMachine.Name,
				"elapsed", time.Since(start),
			)
			return true
		})
		if done == nil {
			instanceScope.Logger.V(1).Info("TCP check still running, requeueing",
				"address", address,
				"machine", instanceScope.MachineScope.StaticMachine.Name,
				"requeueAfter", RequeueForCheckInProgress,
				"taskID", tcpTaskID,
			)
			return ctrl.Result{RequeueAfter: RequeueForCheckInProgress}, nil
		}
		if !*done {
			// A host that does not answer on the ssh port is a transient condition, not a
			// reason to give the StaticInstance back to the pool: the release would write
			// the Pending phase to etcd and the resulting watch event would re-enqueue this
			// very StaticMachine immediately, which is the throttling loop itself. Keep the
			// reservation and retry with the address backoff instead; the bootstrap timeout
			// breaks the cycle if the host never comes back.
			//
			// When counts a failure, so it belongs here and not at the top of the branch:
			// called on every poll of a still running check it would reach the ceiling after
			// a handful of reconciles regardless of how many attempts actually failed.
			delay := c.tcpCheckRateLimiter.When(address)

			instanceScope.Logger.Error(errors.New("Failed to connect via tcp"),
				"Failed to connect via tcp to StaticInstance address", "address", address, "requeueAfter", delay)

			return ctrl.Result{RequeueAfter: delay}, nil
		}

		// Only a successful check resets the backoff. Forgetting unconditionally pins the
		// delay to the base value forever, so the rate limiter never limits anything.
		c.tcpCheckRateLimiter.Forget(address)
	}

	sshCondition := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckSSHCondition)
	if sshCondition == nil || sshCondition.Status != metav1.ConditionTrue {
		sshTaskID := address
		// The task outlives the reconcile that spawned it, so it must not inherit its
		// cancellation. The ssh layer applies its own connect and command timeouts.
		sshCheckCtx := context.WithoutCancel(ctx)

		check := c.checkTaskManager.spawn(taskID(sshTaskID), func() bool {
			start := time.Now()
			status := conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckSSHCondition)
			var sshCl ssh.SSH
			var err error
			if instanceScope.SSHLegacyMode {
				instanceScope.Logger.V(1).Info("using clissh")
				sshCl, err = clissh.CreateSSHClient(instanceScope)
			} else {
				instanceScope.Logger.V(1).Info("using gossh")
				sshCl, err = gossh.CreateSSHClient(instanceScope)
			}
			if err != nil {
				instanceScope.Logger.Error(err, "Failed to set StaticInstance: Failed to connect via ssh")
				return false
			}
			data, err := sshCl.ExecSSHCommandToString(sshCheckCtx, instanceScope, "echo check_ssh")
			if err != nil {
				scanner := bufio.NewScanner(strings.NewReader(data))
				for scanner.Scan() {
					str := scanner.Text()
					if (strings.Contains(str, "Connection to ") && strings.Contains(str, " timed out")) || strings.Contains(str, "Permission denied (publickey).") {
						err := errors.New(str)
						if status == nil || status.Status != metav1.ConditionFalse || status.Reason != err.Error() {
							c.recorder.SendWarningEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "StaticInstanceSshFailed", str)

							instanceScope.Logger.Error(err, "StaticInstance: Failed to connect via ssh")

							conditions.Set(instanceScope.Instance, metav1.Condition{
								Type:               infrav1.StaticInstanceCheckSSHCondition,
								Status:             metav1.ConditionFalse,
								Reason:             err.Error(),
								Message:            err.Error(),
								LastTransitionTime: metav1.Now(),
							})

							err2 := instanceScope.Patch(ctx)
							if err2 != nil {
								instanceScope.Logger.Error(err, "Failed to set StaticInstance: Failed to connect via ssh")
							}
						}
					}
				}
				return false
			}
			if status == nil || status.Status != metav1.ConditionTrue {
				conditions.Set(instanceScope.Instance, metav1.Condition{
					Type:               infrav1.StaticInstanceCheckSSHCondition,
					Status:             metav1.ConditionTrue,
					Reason:             infrav1.StaticInstanceCheckPassedReason,
					Message:            "SSH connectivity check passed",
					LastTransitionTime: metav1.Now(),
				})

				err = instanceScope.Patch(ctx)
				if err != nil {
					instanceScope.Logger.Error(err, "Failed to set StaticInstance: Failed to connect via ssh")
				}
			}
			instanceScope.Logger.Info("SSH connectivity check completed", "address", address, "elapsed", time.Since(start))
			return true
		})
		if check == nil {
			instanceScope.Logger.V(1).Info("SSH check still running, requeueing", "address", address, "requeueAfter", RequeueForCheckInProgress)
			return ctrl.Result{RequeueAfter: RequeueForCheckInProgress}, nil
		}
		if !*check {
			// A host that does not answer over ssh is a transient condition, not a reason to
			// give the StaticInstance back to the pool: the release would write the Pending
			// phase to etcd and the resulting watch event would re-enqueue this very
			// StaticMachine immediately, which is the throttling loop itself. Keep the
			// reservation and retry with the address backoff instead; the bootstrap timeout
			// breaks the cycle if the host never comes back.
			//
			// When counts a failure, so it belongs here and not at the top of the branch:
			// called on every poll of a still running check it would reach the ceiling after
			// a handful of reconciles regardless of how many attempts actually failed.
			delay := c.sshCheckRateLimiter.When(address)

			instanceScope.Logger.Error(errors.New("Failed to connect via ssh"),
				"Failed to connect via ssh to StaticInstance address", "address", address, "requeueAfter", delay)

			return ctrl.Result{RequeueAfter: delay}, nil
		}

		// Only a successful check resets the backoff. Forgetting unconditionally pins the
		// delay to the base value forever, so the rate limiter never limits anything.
		c.sshCheckRateLimiter.Forget(address)
	}

	providerID := providerid.GenerateProviderID(instanceScope.Instance.Name)

	instanceScope.MachineScope.StaticMachine.Spec.ProviderID = providerID

	err := instanceScope.MachineScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to set StaticMachine provider id to '%s'", providerID)
	}

	return ctrl.Result{}, nil
}

func (c *Client) reserveStaticInstance(ctx context.Context, instanceScope *scope.InstanceScope) error {
	currentRef := instanceScope.Instance.Status.MachineRef

	if currentRef != nil && currentRef.UID == instanceScope.MachineScope.StaticMachine.UID {
		if instanceScope.GetPhase() != deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping {
			instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)
			if err := instanceScope.Patch(ctx); err != nil {
				return errors.Wrap(err, "failed to patch StaticInstance phase to Bootstrapping")
			}
		}

		return nil
	}

	if currentRef != nil && currentRef.UID != instanceScope.MachineScope.StaticMachine.UID {
		return errors.Errorf("StaticInstance already reserved for another StaticMachine: %s", currentRef.Name)
	}

	instanceScope.Instance.Status.MachineRef = &corev1.ObjectReference{
		APIVersion: instanceScope.MachineScope.StaticMachine.APIVersion,
		Kind:       instanceScope.MachineScope.StaticMachine.Kind,
		Namespace:  instanceScope.MachineScope.StaticMachine.Namespace,
		Name:       instanceScope.MachineScope.StaticMachine.Name,
		UID:        instanceScope.MachineScope.StaticMachine.UID,
	}

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	if err := instanceScope.Patch(ctx); err != nil {
		if apierrors.IsConflict(err) {
			return errors.Wrap(err, "StaticInstance already reserved by another machine")
		}
		return errors.Wrap(err, "failed to reserve StaticInstance for StaticMachine")
	}

	return nil
}

// setStaticInstancePhaseToRunning finishes the bootstrap process by waiting for bootstrapping Node to appear and patching StaticMachine and StaticInstance.
func (c *Client) setStaticInstancePhaseToRunning(ctx context.Context, instanceScope *scope.InstanceScope) error {
	node, err := getNodeByProviderID(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to get Node by provider id")
	}

	c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "NodeBootstrappingSucceeded", "Node successfully bootstrapped")

	instanceScope.Logger.Info("Node successfully bootstrapped", "node", node.Name)

	instanceScope.MachineScope.StaticMachine.Status.Addresses = mapAddresses(node.Status.Addresses)

	conditions.Set(instanceScope.Instance, metav1.Condition{
		Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             infrav1.StaticInstanceCheckPassedReason,
		Message:            "StaticInstance is ready",
		LastTransitionTime: metav1.Now(),
	})

	instanceScope.Instance.Status.NodeRef = &corev1.ObjectReference{
		APIVersion: node.APIVersion,
		Kind:       node.Kind,
		Name:       node.Name,
		UID:        node.UID,
	}

	conditions.Set(instanceScope.Instance, metav1.Condition{
		Type:               infrav1.StaticInstanceBootstrapSucceededCondition,
		Message:            "StaticInstance is bootstrapped",
		Reason:             infrav1.StaticInstanceBootstrapSucceededCondition,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})

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
	machineAddresses := make([]clusterv1.MachineAddress, 0, len(addresses))

	for _, address := range addresses {
		machineAddresses = append(machineAddresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineAddressType(address.Type),
			Address: address.Address,
		})
	}

	return machineAddresses
}
