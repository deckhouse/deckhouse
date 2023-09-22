package pool

import (
	deckhousev1 "cloud-provider-static/api/deckhouse.io/v1alpha1"
	"cloud-provider-static/internal/scope"
	"context"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"math/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StaticInstancePool defines a pool of static instances.
type StaticInstancePool struct {
	client.Client
	config *rest.Config
}

// NewStaticInstancePool creates a new static instance pool.
func NewStaticInstancePool(client client.Client, config *rest.Config) *StaticInstancePool {
	return &StaticInstancePool{
		Client: client,
		config: config,
	}
}

// PickStaticInstance picks a StaticInstance for the given StaticMachine.
func (p *StaticInstancePool) PickStaticInstance(
	ctx context.Context,
	machineScope *scope.MachineScope,
) (*scope.InstanceScope, bool, error) {
	staticInstances, err := p.findStaticInstancesInPhase(
		ctx,
		machineScope,
		deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
	)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to find static instances in pending phase")
	}
	if len(staticInstances) == 0 {
		return nil, false, nil
	}

	staticInstance := staticInstances[rand.Intn(len(staticInstances))]

	credentials := &deckhousev1.SSHCredentials{}
	credentialsKey := client.ObjectKey{
		Namespace: staticInstance.Namespace,
		Name:      staticInstance.Spec.CredentialsRef.Name,
	}

	err = p.Client.Get(ctx, credentialsKey, credentials)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to get StaticInstance credentials")
	}

	newScope, err := scope.NewScope(p.Client, p.config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create scope")
	}

	instanceScope, err := scope.NewInstanceScope(newScope, &staticInstance)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create instance scope")
	}

	instanceScope.Credentials = credentials
	instanceScope.MachineScope = machineScope

	return instanceScope, true, nil
}

func (p *StaticInstancePool) findStaticInstancesInPhase(
	ctx context.Context,
	machineScope *scope.MachineScope,
	phase deckhousev1.StaticInstanceStatusCurrentStatusPhase,
) ([]deckhousev1.StaticInstance, error) {
	staticInstances := &deckhousev1.StaticInstanceList{}

	labelSelector, err := metav1.LabelSelectorAsSelector(machineScope.StaticMachine.Spec.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert StaticMachine label selector")
	}

	phaseSelector := fields.OneTermEqualSelector("status.currentStatus.phase", string(phase))

	err = p.List(
		ctx,
		staticInstances,
		client.MatchingLabelsSelector{Selector: labelSelector},
		client.MatchingFieldsSelector{Selector: phaseSelector},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find static instances in phase '%s'", phase)
	}

	return staticInstances.Items, nil
}
