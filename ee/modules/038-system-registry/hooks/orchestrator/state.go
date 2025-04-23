/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type State struct {
	Mode registry_const.ModeType `json:"mode,omitempty"`

	PKI          *pki.State          `json:"pki,omitempty"`
	Secrets      *secrets.State      `json:"secrets,omitempty"`
	Users        *users.State        `json:"users,omitempty"`
	NodeServices *nodeservices.State `json:"node_services,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (state *State) setCondition(condition metav1.Condition) {
	existingCondition := state.findCondition(condition.Type)

	if existingCondition == nil {
		// Condition doesn't exist, add it
		condition.LastTransitionTime = metav1.NewTime(time.Now())
		state.Conditions = append(state.Conditions, condition)
		return
	}

	// Only update if something changed
	if existingCondition.Status != condition.Status ||
		existingCondition.Reason != condition.Reason ||
		existingCondition.ObservedGeneration != condition.ObservedGeneration ||
		existingCondition.Message != condition.Message {
		// Status changed, update transition time
		if existingCondition.Status != condition.Status {
			condition.LastTransitionTime = metav1.NewTime(time.Now())
		} else {
			condition.LastTransitionTime = existingCondition.LastTransitionTime
		}

		// Replace the existing condition
		*existingCondition = condition
	}
}

func (state *State) findCondition(conditionType string) *metav1.Condition {
	for i := range state.Conditions {
		if state.Conditions[i].Type == conditionType {
			return &state.Conditions[i]
		}
	}
	return nil
}

func (state *State) removeCondition(conditionType string) {
	newConditions := make([]metav1.Condition, 0, len(state.Conditions))
	for _, c := range state.Conditions {
		if c.Type != conditionType {
			newConditions = append(newConditions, c)
		}
	}
	state.Conditions = newConditions
}

func (state *State) isConditionTrue(conditionType string) bool {
	condition := state.findCondition(conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func (state *State) clearConditions() {
	state.Conditions = nil
}

func (state *State) process(log go_hook.Logger, inputs Inputs) error {
	params := inputs.Params

	if params.Mode == "" {
		params.Mode = registry_const.ModeUnmanaged
	} else if params.Mode == registry_const.ModeDetached {
		params.Mode = registry_const.ModeLocal
	}

	if params.Mode != state.Mode {
		log.Warn(
			"Mode change",
			"old_mode", state.Mode,
			"new_mode", params.Mode,
		)

		// TODO: it is clear conditions on each reconcillation during transition phase
		state.clearConditions()
	}

	inputs.Params = params

	switch params.Mode {
	case registry_const.ModeLocal:
		return state.transitionToLocal(log, inputs)
	case registry_const.ModeProxy:
		return state.transitionToProxy(log, inputs)
	case registry_const.ModeDirect:
		return state.transitionToDirect(log, inputs)
	case registry_const.ModeUnmanaged:
		return state.transitionToUnmanaged(log, inputs)
	default:
		return fmt.Errorf("unsupported mode: %v", params.Mode)
	}
}

func (state *State) transitionToLocal(log go_hook.Logger, inputs Inputs) error {
	if state.Mode == registry_const.ModeProxy {
		return ErrTransitionNotSupported{
			From: state.Mode,
			To:   inputs.Params.Mode,
		}
	}

	// PKI
	if state.PKI == nil {
		state.PKI = &inputs.PKI
	}

	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypePKI,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Secrets
	if state.Secrets == nil {
		state.Secrets = &inputs.Secrets
	}

	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeSecrets,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Users
	if state.Users == nil {
		state.Users = &users.State{}
	}

	usersParams := state.Users.GetParams()
	usersParams.RO = true
	usersParams.RW = true
	usersParams.Mirrorer = true

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeUsers,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// NodeServices
	if state.NodeServices == nil {
		state.NodeServices = &nodeservices.State{}
	}

	nodeservicesParams := nodeservices.Params{
		CA:         pkiResult.CA,
		Token:      pkiResult.Token,
		HTTPSecret: state.Secrets.HTTP,
		UserRO:     *state.Users.RO,
		Local: &nodeservices.LocalModeParams{
			UserRW:     *state.Users.RW,
			UserPuller: *state.Users.MirrorPuller,
			UserPusher: *state.Users.MirrorPusher,
		},
	}

	if inputs.IngressCA != "" {
		cert, err := registry_pki.DecodeCertificate([]byte(inputs.IngressCA))
		if err != nil {
			return fmt.Errorf("cannot decode Ingress CA: %w", err)
		}

		nodeservicesParams.Local.IngressCA = cert
	}

	nodeServicesResult, err := state.NodeServices.Process(log, nodeservicesParams, inputs.NodeServices)
	if err != nil {
		return fmt.Errorf("cannot process NodeServices: %w", err)
	}

	nodeServicesMessage := nodeServicesResult.GetConditionMessage()
	if nodeServicesMessage != "" {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeNodeServices,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            nodeServicesMessage,
		})

		state.setReadyCondition(false, inputs)
		return nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServices,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
		Reason:             ConditionReasonReady,
		Message:            nodeServicesMessage,
	})

	// TODO: configure ingress

	// TODO: check images in local registry

	// TODO: configure bashible

	// TODO: service switch

	// TODO: update deckhouse-registry secret

	// Cleanup

	// TODO: stop in-cluster proxy

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	state.Mode = inputs.Params.Mode
	state.setCondition(metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return nil
}

func (state *State) transitionToProxy(log go_hook.Logger, inputs Inputs) error {
	if state.Mode == registry_const.ModeLocal {
		return ErrTransitionNotSupported{
			From: state.Mode,
			To:   inputs.Params.Mode,
		}
	}

	// PKI
	if state.PKI == nil {
		state.PKI = &inputs.PKI
	}

	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypePKI,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Secrets
	if state.Secrets == nil {
		state.Secrets = &inputs.Secrets
	}

	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeSecrets,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Users
	if state.Users == nil {
		state.Users = &users.State{}
	}

	usersParams := state.Users.GetParams()
	usersParams.RO = true

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeUsers,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// TODO: check images in remote registry

	// NodeServices
	if state.NodeServices == nil {
		state.NodeServices = &nodeservices.State{}
	}

	nodeservicesParams := nodeservices.Params{
		CA:         pkiResult.CA,
		Token:      pkiResult.Token,
		HTTPSecret: state.Secrets.HTTP,
		UserRO:     *state.Users.RO,
		Proxy: &nodeservices.ProxyModeParams{
			Scheme:     inputs.Params.Scheme,
			ImagesRepo: inputs.Params.ImagesRepo,
			UserName:   inputs.Params.UserName,
			Password:   inputs.Params.Password,
			TTL:        inputs.Params.TTL,
		},
	}

	if inputs.Params.CA != "" {
		cert, err := registry_pki.DecodeCertificate([]byte(inputs.Params.CA))
		if err != nil {
			log.Error("Cannot decode upstream CA", "error", err)

			state.setCondition(metav1.Condition{
				Type:               ConditionTypeNodeServices,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: inputs.Params.Generation,
				Reason:             ConditionReasonError,
				Message:            fmt.Sprintf("Cannot decode upstream CA: %v", err),
			})

			state.setReadyCondition(false, inputs)
			return nil
		}

		nodeservicesParams.Proxy.UpstreamCA = cert
	}

	nodeServicesResult, err := state.NodeServices.Process(log, nodeservicesParams, inputs.NodeServices)
	if err != nil {
		return fmt.Errorf("cannot process NodeServices: %w", err)
	}

	nodeServicesMessage := nodeServicesResult.GetConditionMessage()
	if nodeServicesMessage != "" {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeNodeServices,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            nodeServicesMessage,
		})

		state.setReadyCondition(false, inputs)
		return nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServices,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
		Reason:             ConditionReasonReady,
		Message:            nodeServicesMessage,
	})

	// TODO: configure bashible

	// TODO: service switch

	// TODO: update deckhouse-registry secret

	// Cleanup
	usersParams = users.Params{
		RO: true,
	}

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	// TODO: stop in-cluster proxy

	// TODO: stop ingress from Local Mode

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	state.Mode = inputs.Params.Mode
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToDirect(log go_hook.Logger, inputs Inputs) error {
	// PKI
	if state.PKI == nil {
		state.PKI = &inputs.PKI
	}

	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypePKI,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Secrets
	if state.Secrets == nil {
		state.Secrets = &inputs.Secrets
	}

	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeSecrets,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// Users
	if state.Users == nil {
		state.Users = &users.State{}
	}

	// TODO: configure in-cluster proxy
	_ = pkiResult

	// TODO: check images in remote registry

	// TODO: configure bashible

	// TODO: service switch

	// TODO: update deckhouse-registry secret

	// Cleanup
	state.Users = nil
	state.NodeServices = nil

	// TODO: stop ingres from Local Mode

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	state.Mode = inputs.Params.Mode
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToUnmanaged(log go_hook.Logger, inputs Inputs) error {
	// TODO: check images in remote registry

	// TODO: configure bashible

	// TODO: update deckhouse-registry secret

	// Cleanup
	state.PKI = nil
	state.Secrets = nil
	state.Users = nil
	state.NodeServices = nil

	// TODO: remove service

	// TODO: stop ingres from Local Mode

	// TODO: stop in-cluster proxy

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	state.Mode = inputs.Params.Mode
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) setReadyCondition(ready bool, inputs Inputs) {
	if ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: inputs.Params.Generation,
		})
	} else {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            fmt.Sprintf("Transitioning to %v", inputs.Params.Mode),
		})
	}
}
