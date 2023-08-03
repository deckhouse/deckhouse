package bootstrap

import (
	infrav1 "cloud-provider-static/api/v1alpha1"
	"cloud-provider-static/internal/providerid"
	"cloud-provider-static/internal/scope"
	"cloud-provider-static/internal/ssh"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Bootstrap runs the bootstrap script on StaticInstance.
func Bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) error {
	if instanceScope.GetPhase() != infrav1.StaticInstanceStatusCurrentStatusPhasePending {
		return errors.New("StaticInstance is not pending")
	}

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

	instanceScope.SetPhase(infrav1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	err = instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance MachineRef and Phase")
	}

	err = bootstrap(ctx, instanceScope)
	if err != nil {
		return err
	}

	return nil
}

func bootstrap(ctx context.Context, instanceScope *scope.InstanceScope) error {
	providerID := instanceScope.MachineScope.StaticMachine.Spec.ProviderID

	bootstrapScript, err := GetBootstrapScript(ctx, instanceScope)
	if err != nil {
		return errors.Wrap(err, "failed to get bootstrap script")
	}

	go func() {
		err := ssh.ExecSSHCommand(instanceScope, fmt.Sprintf("mkdir /var/lib/bashible && echo '%s' > /var/lib/bashible/node-spec-provider-id && echo '%s' | base64 -d | bash", providerID, base64.StdEncoding.EncodeToString(bootstrapScript)), nil)
		if err != nil {
			// If the node reboots, the ssh connection will close, and we will get an error.
			instanceScope.Logger.Error(err, "Failed to bootstrap StaticInstance: failed to exec ssh command")
		}
	}()

	return nil
}

// FinishBootstrapping finishes the bootstrap process by waiting for bootstrapping Node to appear and patching StaticMachine and StaticInstance.
func FinishBootstrapping(ctx context.Context, instanceScope *scope.InstanceScope) error {
	err := bootstrap(ctx, instanceScope)
	if err != nil {
		return err
	}

	providerID, err := ssh.ExecSSHCommandToString(instanceScope, "cat /var/lib/bashible/node-spec-provider-id")
	if err != nil {
		return errors.Wrap(err, "failed to read /var/lib/bashible/node-spec-provider-id")
	}

	err = providerid.ValidateProviderID(providerID)
	if err != nil {
		return errors.Wrapf(err, "failed to validate provider id '%s'", providerID)
	}

	node, err := WaitForNode(ctx, instanceScope, providerID)
	if err != nil {
		return errors.Wrap(err, "failed to wait for Node to appear")
	}

	instanceScope.Logger.Info("Node successfully bootstrapped", "node", node.Name)

	err = PatchNode(ctx, instanceScope.MachineScope, node)
	if err != nil {
		return errors.Wrap(err, "failed to patch Node with StaticMachine labels, annotations and taints")
	}

	instanceScope.MachineScope.StaticMachine.Spec.ProviderID = node.Spec.ProviderID
	instanceScope.MachineScope.StaticMachine.Status.Addresses = mapAddresses(node.Status.Addresses)
	instanceScope.MachineScope.StaticMachine.Status.Ready = true

	conditions.MarkTrue(instanceScope.MachineScope.StaticMachine, infrav1.StaticMachineStaticInstanceReadyCondition)

	instanceScope.Instance.Status.NodeRef = &corev1.ObjectReference{
		APIVersion: node.APIVersion,
		Kind:       node.Kind,
		Name:       node.Name,
		UID:        node.UID,
	}

	conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition)

	instanceScope.SetPhase(infrav1.StaticInstanceStatusCurrentStatusPhaseRunning)

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

// WaitForNode waits for the node to appear and checks that it has 'node.deckhouse.io/configuration-checksum' annotation.
func WaitForNode(ctx context.Context, instanceScope *scope.InstanceScope, providerID string) (*corev1.Node, error) {
	nodes := &corev1.NodeList{}
	nodeSelector := fields.OneTermEqualSelector("spec.providerID", providerID)

	err := instanceScope.Client.List(
		ctx,
		nodes,
		client.MatchingFieldsSelector{Selector: nodeSelector},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Node by provider id '%s'", providerID)
	}

	if len(nodes.Items) == 0 {
		return nil, errors.Errorf("Node with provider id '%s' not found", providerID)
	}

	if len(nodes.Items) > 1 {
		return nil, errors.Errorf("found more than one Node with provider id '%s'", providerID)
	}

	node := &nodes.Items[0]

	if node.Annotations["node.deckhouse.io/configuration-checksum"] == "" {
		return nil, errors.Errorf("Node '%s' doesn't have 'node.deckhouse.io/configuration-checksum' annotation", node.Name)
	}

	return node, nil
}

// PatchNode patches Node with StaticMachine labels, annotations and taints.
func PatchNode(ctx context.Context, machineScope *scope.MachineScope, node *corev1.Node) error {
	patchHelper, err := patch.NewHelper(node, machineScope.Client)
	if err != nil {
		return errors.Wrap(err, "failed to init Node patch helper")
	}

	if len(machineScope.StaticMachine.Spec.Labels) > 0 && node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	for name, value := range machineScope.StaticMachine.Spec.Labels {
		node.Labels[name] = value
	}

	if len(machineScope.StaticMachine.Spec.Annotations) > 0 && node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	for name, value := range machineScope.StaticMachine.Spec.Annotations {
		node.Annotations[name] = value
	}

	node.Spec.Taints = append(node.Spec.Taints, machineScope.StaticMachine.Spec.Taints...)

	err = patchHelper.Patch(ctx, node)
	if err != nil {
		return errors.Wrap(err, "failed to patch Node")
	}

	return nil
}

// GetBootstrapScript returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func GetBootstrapScript(ctx context.Context, instanceScope *scope.InstanceScope) ([]byte, error) {
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
