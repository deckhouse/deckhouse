/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	registrysecret "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-secret"
	registryservice "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-service"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type State struct {
	ActualParams Params                  `json:"actual_params,omitempty"`
	TargetMode   registry_const.ModeType `json:"target_mode,omitempty"`

	PKI             pki.State            `json:"pki,omitempty"`
	Secrets         secrets.State        `json:"secrets,omitempty"`
	Users           users.State          `json:"users,omitempty"`
	NodeServices    nodeservices.State   `json:"node_services,omitempty"`
	InClusterProxy  inclusterproxy.State `json:"in_cluster_proxy,omitempty"`
	IngressEnabled  bool                 `json:"ingress_enabled,omitempty"`
	RegistryService registryservice.Mode `json:"registry_service,omitempty"`
	Bashible        bashible.State       `json:"bashible,omitempty"`
	RegistrySecret  registrysecret.State `json:"-"`

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
	switch inputs.Params.Mode {
	case "":
		inputs.Params.Mode = registry_const.ModeUnmanaged
	case registry_const.ModeDetached:
		inputs.Params.Mode = registry_const.ModeLocal
	}

	if state.TargetMode != inputs.Params.Mode {
		state.TargetMode = inputs.Params.Mode

		log.Warn(
			"Mode change",
			"mode", state.ActualParams.Mode,
			"target_mode", state.TargetMode,
		)

		state.clearConditions()
	}

	switch state.TargetMode {
	case registry_const.ModeLocal:
		return state.transitionToLocal(log, inputs)
	case registry_const.ModeProxy:
		return state.transitionToProxy(log, inputs)
	case registry_const.ModeDirect:
		return state.transitionToDirect(log, inputs)
	case registry_const.ModeUnmanaged:
		return state.transitionToUnmanaged(log, inputs)
	default:
		return fmt.Errorf("unsupported mode: %v", state.TargetMode)
	}
}

func (state *State) transitionToLocal(log go_hook.Logger, inputs Inputs) error {
	if state.ActualParams.Mode == registry_const.ModeProxy {
		return ErrTransitionNotSupported{
			From: state.ActualParams.Mode,
			To:   state.TargetMode,
		}
	}

	// PKI
	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	// Secrets
	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	// Users
	usersParams := state.Users.GetParams()
	usersParams.RO = true
	usersParams.RW = true
	usersParams.Mirrorer = true

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	state.IngressEnabled = true

	// NodeServices
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

	if inputs.IngressClientCA != "" {
		cert, err := registry_pki.DecodeCertificate([]byte(inputs.IngressClientCA))
		if err != nil {
			return fmt.Errorf("cannot decode Ingress client CA: %w", err)
		}

		nodeservicesParams.Local.IngressClientCA = cert
	}

	nodeServicesResult, err := state.NodeServices.Process(log, nodeservicesParams, inputs.NodeServices)
	if err != nil {
		return fmt.Errorf("cannot process NodeServices: %w", err)
	}

	if !nodeServicesResult.IsReady() {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeNodeServices,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            nodeServicesResult.GetConditionMessage(),
		})

		state.setReadyCondition(false, inputs)
		return nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServices,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// TODO: check images in local registry

	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Mode: state.TargetMode,
			ProxyLocal: &bashible.ProxyLocalModeParams{
				CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
				Username: state.Users.RO.UserName,
				Password: state.Users.RO.Password,
			},
		},
	}
	registrySecretParams := registrysecret.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registrysecret.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: state.Users.RO.UserName,
			Password: state.Users.RO.Password,
		},
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleFirstStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeNodeServices

	// Deckhouse-registry secret
	processedRegistrySecret, err := state.RegistrySecret.Process(registrySecretParams)
	if err != nil {
		return err
	}
	if !processedRegistrySecret {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleSecondStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Cleanup
	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	if !inClusterProxyReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// All done
	state.ActualParams = inputs.Params
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToProxy(log go_hook.Logger, inputs Inputs) error {
	if state.ActualParams.Mode == registry_const.ModeLocal {
		return ErrTransitionNotSupported{
			From: state.ActualParams.Mode,
			To:   state.TargetMode,
		}
	}

	// PKI
	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	// Secrets
	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	// Users
	usersParams := state.Users.GetParams()
	usersParams.RO = true

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	// NodeServices
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
	})

	// TODO: check images in remote registry via proxy

	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Mode: state.TargetMode,
			ProxyLocal: &bashible.ProxyLocalModeParams{
				CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
				Username: state.Users.RO.UserName,
				Password: state.Users.RO.Password,
			},
		},
	}
	registrySecretParams := registrysecret.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registrysecret.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: state.Users.RO.UserName,
			Password: state.Users.RO.Password,
		},
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleFirstStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeNodeServices

	// Deckhouse-registry secret
	processedRegistrySecret, err := state.RegistrySecret.Process(registrySecretParams)
	if err != nil {
		return err
	}
	if !processedRegistrySecret {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleSecondStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Cleanup
	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	state.IngressEnabled = false

	if !inClusterProxyReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	usersParams = users.Params{
		RO: true,
	}

	if err := state.Users.Process(usersParams, inputs.Users); err != nil {
		return fmt.Errorf("cannot process Users: %w", err)
	}

	// All done
	state.ActualParams = inputs.Params
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToDirect(log go_hook.Logger, inputs Inputs) error {
	// PKI
	pkiResult, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	// Secrets
	if err := state.Secrets.Process(); err != nil {
		return fmt.Errorf("cannot process Secrets: %w", err)
	}

	// Configure in-cluster proxy
	inClusterProxyParams := inclusterproxy.Params{
		CA:         pkiResult.CA,
		Token:      pkiResult.Token,
		HTTPSecret: state.Secrets.HTTP,
		Upstream: inclusterproxy.UpstreamParams{
			Scheme:     inputs.Params.Scheme,
			ImagesRepo: inputs.Params.ImagesRepo,
			UserName:   inputs.Params.UserName,
			Password:   inputs.Params.Password,
		},
	}

	if inputs.Params.CA != "" {
		cert, err := registry_pki.DecodeCertificate([]byte(inputs.Params.CA))
		if err != nil {
			log.Error("Cannot decode upstream CA", "error", err)

			state.setCondition(metav1.Condition{
				Type:               ConditionTypeInClusterProxy,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: inputs.Params.Generation,
				Reason:             ConditionReasonError,
				Message:            fmt.Sprintf("Cannot decode upstream CA: %v", err),
			})

			state.setReadyCondition(false, inputs)
			return nil
		}

		inClusterProxyParams.Upstream.CA = cert
	}

	inClusterProxyProcessResult, err := state.InClusterProxy.Process(log, inClusterProxyParams, inputs.InClusterProxy)
	if err != nil {
		return fmt.Errorf("cannot process InClusterProxy: %w", err)
	}

	if !inClusterProxyProcessResult.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeInClusterProxy,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            inClusterProxyProcessResult.Message,
		})

		state.setReadyCondition(false, inputs)
		return nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeInClusterProxy,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	// TODO: check images in remote registry

	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Mode: state.TargetMode,
			Direct: &bashible.DirectModeParams{
				ImagesRepo: inputs.Params.ImagesRepo,
				Scheme:     inputs.Params.Scheme,
				CA:         inputs.Params.CA,
				Username:   inputs.Params.UserName,
				Password:   inputs.Params.Password,
			},
		},
	}
	registrySecretParams := registrysecret.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registrysecret.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: inputs.Params.UserName,
			Password: inputs.Params.Password,
		},
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleFirstStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeInClusterProxy

	// Deckhouse-registry secret
	processedRegistrySecret, err := state.RegistrySecret.Process(registrySecretParams)
	if err != nil {
		return err
	}
	if !processedRegistrySecret {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleSecondStage(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Cleanup
	nodeServicesReady, err := state.cleanupNodeServices(inputs)
	if err != nil {
		return fmt.Errorf("cannot cleanup NodeServices: %w", err)
	}

	state.IngressEnabled = false

	if !nodeServicesReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	state.Users = users.State{}

	// All done
	state.ActualParams = inputs.Params
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToUnmanaged(log go_hook.Logger, inputs Inputs) error {
	_ = log
	if state.ActualParams.Mode != registry_const.ModeUnmanaged &&
		state.ActualParams.Mode != registry_const.ModeProxy &&
		state.ActualParams.Mode != registry_const.ModeDirect {
		return ErrTransitionNotSupported{
			From: state.ActualParams.Mode,
			To:   state.TargetMode,
		}
	}

	// TODO: check images in remote registry

	// Bashible with actual params
	if (state.ActualParams.Mode == registry_const.ModeProxy ||
		state.ActualParams.Mode == registry_const.ModeDirect) &&
		!state.Bashible.IsStopped() {
		bashibleParams := bashible.Params{
			RegistrySecret: inputs.RegistrySecret,
			ModeParams: bashible.ModeParams{
				Mode: state.TargetMode,
				Unmanaged: &bashible.UnmanagedModeParams{
					ImagesRepo: state.ActualParams.ImagesRepo,
					Scheme:     state.ActualParams.Scheme,
					CA:         state.ActualParams.CA,
					Username:   state.ActualParams.UserName,
					Password:   state.ActualParams.Password,
				},
			},
		}

		processed, err := state.processBashibleFirstStage(bashibleParams, inputs)
		if err != nil {
			return err
		}
		if !processed {
			state.setReadyCondition(false, inputs)
			return nil
		}
	}

	// Deckhouse-registry secret
	if state.ActualParams.Mode == registry_const.ModeProxy ||
		state.ActualParams.Mode == registry_const.ModeDirect {
		registrySecretParams := registrysecret.Params{
			RegistrySecret: inputs.RegistrySecret,
			UnmanagedMode: &registrysecret.UnmanagedModeParams{
				ImagesRegistry: state.ActualParams.ImagesRepo,
				Scheme:         state.ActualParams.Scheme,
				CA:             state.ActualParams.CA,
				Username:       state.ActualParams.UserName,
				Password:       state.ActualParams.Password,
			},
		}

		processed, err := state.RegistrySecret.Process(registrySecretParams)
		if err != nil {
			return err
		}
		if !processed {
			state.setReadyCondition(false, inputs)
			return nil
		}
	}

	// Cleanup
	bashibleReady := state.cleanupBashible(inputs)

	nodeServicesReady, err := state.cleanupNodeServices(inputs)
	if err != nil {
		return fmt.Errorf("cannot cleanup NodeServices: %w", err)
	}
	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	state.RegistryService = registryservice.ModeDisabled

	state.IngressEnabled = false

	if !bashibleReady {
		state.setReadyCondition(false, inputs)
		return nil
	}
	if !nodeServicesReady {
		state.setReadyCondition(false, inputs)
		return nil
	}
	if !inClusterProxyReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	state.PKI = pki.State{}
	state.Secrets = secrets.State{}
	state.Users = users.State{}

	// All done
	state.ActualParams = inputs.Params
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) processBashibleFirstStage(params bashible.Params, inputs Inputs) (bool, error) {
	processResult, err := state.Bashible.Process(params, inputs.Bashible, true)
	if err != nil {
		return false, fmt.Errorf("cannot process Bashible: %w", err)
	}

	if !processResult.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashible,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            processResult.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashible,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) processBashibleSecondStage(params bashible.Params, inputs Inputs) (bool, error) {
	processResult, err := state.Bashible.Process(params, inputs.Bashible, false)
	if err != nil {
		return false, fmt.Errorf("cannot process Bashible: %w", err)
	}

	if !processResult.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashible,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            processResult.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashible,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) cleanupBashible(inputs Inputs) bool {
	result := state.Bashible.Stop(inputs.Bashible)

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashible,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashible,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true
}

func (state *State) cleanupNodeServices(inputs Inputs) (bool, error) {
	nodes, err := state.NodeServices.Stop(inputs.NodeServices)
	if err != nil {
		return false, fmt.Errorf("cannot stop: %w", err)
	}

	if len(nodes) > 0 {
		sort.Strings(nodes)

		builder := new(strings.Builder)

		fmt.Fprintln(builder, "Waiting for nodes cleanup:")

		for _, name := range nodes {
			fmt.Fprintf(builder, "- %v\n", name)
		}

		state.setCondition(metav1.Condition{
			Type:               ConditionTypeNodeServices,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            builder.String(),
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServices,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return true, nil
}

func (state *State) cleanupInClusterProxy(inputs Inputs) bool {
	inClusterProxyStopResult := state.InClusterProxy.Stop(inputs.InClusterProxy)

	if !inClusterProxyStopResult.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeInClusterProxy,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            inClusterProxyStopResult.Message,
		})
		return false
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeInClusterProxy,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return true
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
			Message:            fmt.Sprintf("Transitioning to %v", state.TargetMode),
		})
	}
}
