/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"crypto/x509"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-service"
	registryswitcher "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-switcher"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type State struct {
	Mode       registry_const.ModeType `json:"mode,omitempty"`
	TargetMode registry_const.ModeType `json:"target_mode,omitempty"`

	PKI             pki.State              `json:"pki,omitempty"`
	Secrets         secrets.State          `json:"secrets,omitempty"`
	Users           users.State            `json:"users,omitempty"`
	NodeServices    nodeservices.State     `json:"node_services,omitempty"`
	InClusterProxy  inclusterproxy.State   `json:"in_cluster_proxy,omitempty"`
	IngressEnabled  bool                   `json:"ingress_enabled,omitempty"`
	RegistryService registryservice.Mode   `json:"registry_service,omitempty"`
	Bashible        bashible.State         `json:"bashible,omitempty"`
	RegistrySecret  registryswitcher.State `json:"-"`
	CheckerParams   checker.Params         `json:"-"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (state *State) setCondition(condition metav1.Condition) {
	var existingCondition *metav1.Condition
	conditions := make([]metav1.Condition, 0, len(state.Conditions)+1)

	for _, cond := range state.Conditions {
		if cond.Type == condition.Type {
			c := cond // make local copy
			existingCondition = &c
			continue
		}

		conditions = append(conditions, cond)
	}

	if existingCondition != nil {
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
	} else {
		// Condition doesn't exist, will add it
		condition.LastTransitionTime = metav1.NewTime(time.Now())
		existingCondition = &condition
	}

	conditions = append(conditions, *existingCondition)
	state.Conditions = conditions
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
			"mode", state.Mode,
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
	if state.Mode == registry_const.ModeProxy {
		return ErrTransitionNotSupported{
			From: state.Mode,
			To:   state.TargetMode,
		}
	}

	// Preflight Checks
	if !state.bashiblePreflightCheck(inputs) {
		state.setReadyCondition(false, inputs)
		return nil
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

	nodeservicesParams := nodeservices.Params{
		CA:         pkiResult.CA,
		Token:      pkiResult.Token,
		HTTPSecret: state.Secrets.HTTP,
		UserRO:     *state.Users.RO,
		Local: &nodeservices.LocalModeParams{
			UserRW:          *state.Users.RW,
			UserPuller:      *state.Users.MirrorPuller,
			UserPusher:      *state.Users.MirrorPusher,
			IngressClientCA: inputs.IngressClientCA,
		},
	}
	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Local: &bashible.ProxyLocalModeParams{
				CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
				Username: state.Users.RO.UserName,
				Password: state.Users.RO.Password,
			},
		},
	}
	registrySecretParams := registryswitcher.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registryswitcher.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: state.Users.RO.UserName,
			Password: state.Users.RO.Password,
		},
	}

	// Ingress
	state.IngressEnabled = true

	// NodeServices
	nodeServicesResult, err := state.processNodeServices(log, nodeservicesParams, inputs)
	if err != nil {
		return err
	}
	if !nodeServicesResult.IsReady() {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Check images in nodese registries
	checkerReady, err := state.processCheckerNodes(
		inputs,
		nodeServicesResult,
		nodeservicesParams.UserRO,
		nodeservicesParams.CA.Cert,
	)
	if err != nil {
		return fmt.Errorf("cannot process checkper on upstream: %w", err)
	}

	if !checkerReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleTransition(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeNodeServices

	// Update Deckhouse-registry secret and wait
	processedRegistrySwitcher, err := state.processRegistrySwitcher(registrySecretParams, inputs)
	if err != nil {
		return err
	}
	if !processedRegistrySwitcher {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleFinalize(bashibleParam, inputs)
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
	state.Mode = state.TargetMode
	state.Bashible.UnmanagedParams = nil
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToProxy(log go_hook.Logger, inputs Inputs) error {
	if state.Mode == registry_const.ModeLocal {
		return ErrTransitionNotSupported{
			From: state.Mode,
			To:   state.TargetMode,
		}
	}

	// check upstream registry
	checkerRegistryParams := checker.RegistryParams{
		Address:  inputs.Params.ImagesRepo,
		Scheme:   strings.ToUpper(inputs.Params.Scheme),
		Username: inputs.Params.UserName,
		Password: inputs.Params.Password,
	}
	if inputs.Params.CA != nil {
		checkerRegistryParams.CA = string(registry_pki.EncodeCertificate(inputs.Params.CA))
	}

	checkerReady, err := state.processCheckerUpstream(checkerRegistryParams, inputs)
	if err != nil {
		return fmt.Errorf("cannot process checkper on upstream: %w", err)
	}

	if !checkerReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Preflight Checks
	if !state.bashiblePreflightCheck(inputs) {
		state.setReadyCondition(false, inputs)
		return nil
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
			UpstreamCA: inputs.Params.CA,
		},
	}
	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Proxy: &bashible.ProxyLocalModeParams{
				CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
				Username: state.Users.RO.UserName,
				Password: state.Users.RO.Password,
			},
		},
	}
	registrySecretParams := registryswitcher.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registryswitcher.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: state.Users.RO.UserName,
			Password: state.Users.RO.Password,
		},
	}

	// NodeServices
	nodeServicesResult, err := state.processNodeServices(log, nodeservicesParams, inputs)
	if err != nil {
		return err
	}
	if !nodeServicesResult.IsReady() {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleTransition(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeNodeServices

	// Update Deckhouse-registry secret and wait
	processedRegistrySwitcher, err := state.processRegistrySwitcher(registrySecretParams, inputs)
	if err != nil {
		return err
	}
	if !processedRegistrySwitcher {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleFinalize(bashibleParam, inputs)
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
	state.Mode = state.TargetMode
	state.Bashible.UnmanagedParams = &bashible.UnmanagedModeParams{
		ImagesRepo: inputs.Params.ImagesRepo,
		Scheme:     inputs.Params.Scheme,
		CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
		Username:   inputs.Params.UserName,
		Password:   inputs.Params.Password,
	}
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToDirect(log go_hook.Logger, inputs Inputs) error {
	// check upstream registry
	checkerRegistryParams := checker.RegistryParams{
		Address:  inputs.Params.ImagesRepo,
		Scheme:   strings.ToUpper(inputs.Params.Scheme),
		Username: inputs.Params.UserName,
		Password: inputs.Params.Password,
	}
	if inputs.Params.CA != nil {
		checkerRegistryParams.CA = string(registry_pki.EncodeCertificate(inputs.Params.CA))
	}

	checkerReady, err := state.processCheckerUpstream(checkerRegistryParams, inputs)
	if err != nil {
		return fmt.Errorf("cannot process checkper on upstream: %w", err)
	}

	if !checkerReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Preflight Checks
	if !state.bashiblePreflightCheck(inputs) {
		state.setReadyCondition(false, inputs)
		return nil
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

	inClusterProxyParams := inclusterproxy.Params{
		CA:         pkiResult.CA,
		Token:      pkiResult.Token,
		HTTPSecret: state.Secrets.HTTP,
		Upstream: inclusterproxy.UpstreamParams{
			Scheme:     inputs.Params.Scheme,
			ImagesRepo: inputs.Params.ImagesRepo,
			UserName:   inputs.Params.UserName,
			Password:   inputs.Params.Password,
			CA:         inputs.Params.CA,
		},
	}
	bashibleParam := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Direct: &bashible.DirectModeParams{
				ImagesRepo: inputs.Params.ImagesRepo,
				Scheme:     inputs.Params.Scheme,
				CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
				Username:   inputs.Params.UserName,
				Password:   inputs.Params.Password,
			},
		},
	}
	registrySecretParams := registryswitcher.Params{
		RegistrySecret: inputs.RegistrySecret,
		ManagedMode: &registryswitcher.ManagedModeParams{
			CA:       string(registry_pki.EncodeCertificate(pkiResult.CA.Cert)),
			Username: inputs.Params.UserName,
			Password: inputs.Params.Password,
		},
	}

	// Bashible with actual params
	processedBashible, err := state.processBashibleTransition(bashibleParam, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Configure in-cluster proxy

	// When switching from Direct to Direct mode, we must wait until the new configuration
	// is fully applied on the nodes (i.e., Bashible has finished running).
	// This is necessary because during a Direct → Direct transition, the RPP still uses
	// the old InclusterProxy, which must not be overwritten prematurely.
	processedInClusterProxy, err := state.processInClusterProxy(log, inClusterProxyParams, inputs)
	if err != nil {
		return err
	}
	if !processedInClusterProxy {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Registry service
	state.RegistryService = registryservice.ModeInClusterProxy

	// Update Deckhouse-registry secret and wait
	processedRegistrySwitcher, err := state.processRegistrySwitcher(registrySecretParams, inputs)
	if err != nil {
		return err
	}
	if !processedRegistrySwitcher {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Bashible only input params
	processedBashible, err = state.processBashibleFinalize(bashibleParam, inputs)
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
	state.Mode = state.TargetMode
	state.Bashible.UnmanagedParams = &bashible.UnmanagedModeParams{
		ImagesRepo: inputs.Params.ImagesRepo,
		Scheme:     inputs.Params.Scheme,
		CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
		Username:   inputs.Params.UserName,
		Password:   inputs.Params.Password,
	}
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) transitionToUnmanaged(log go_hook.Logger, inputs Inputs) error {
	_ = log
	if state.Mode != registry_const.ModeUnmanaged &&
		state.Mode != registry_const.ModeProxy &&
		state.Mode != registry_const.ModeDirect {
		return ErrTransitionNotSupported{
			From: state.Mode,
			To:   state.TargetMode,
		}
	}

	// Reset checker
	state.CheckerParams = checker.Params{}

	if (state.Mode == registry_const.ModeProxy ||
		state.Mode == registry_const.ModeDirect) &&
		state.Bashible.UnmanagedParams != nil {
		unmanagedParams := *state.Bashible.UnmanagedParams

		bashibleParams := bashible.Params{
			RegistrySecret: inputs.RegistrySecret,
			ModeParams: bashible.ModeParams{
				Unmanaged: &unmanagedParams,
			},
		}

		registrySecretParams := registryswitcher.Params{
			RegistrySecret: inputs.RegistrySecret,
			UnmanagedMode: &registryswitcher.UnmanagedModeParams{
				ImagesRepo: unmanagedParams.ImagesRepo,
				Scheme:     unmanagedParams.Scheme,
				CA:         unmanagedParams.CA,
				Username:   unmanagedParams.Username,
				Password:   unmanagedParams.Password,
			},
		}

		// check upstream registry
		checkerRegistryParams := checker.RegistryParams{
			Address:  unmanagedParams.ImagesRepo,
			Scheme:   strings.ToUpper(unmanagedParams.Scheme),
			CA:       unmanagedParams.CA,
			Username: unmanagedParams.Username,
			Password: unmanagedParams.Username,
		}

		checkerReady, err := state.processCheckerUpstream(checkerRegistryParams, inputs)
		if err != nil {
			return fmt.Errorf("cannot process checkper on upstream: %w", err)
		}

		if !checkerReady {
			state.setReadyCondition(false, inputs)
			return nil
		}

		// Bashible with actual params
		processedBashible, err := state.processBashibleTransition(bashibleParams, inputs)
		if err != nil {
			return err
		}
		if !processedBashible {
			state.setReadyCondition(false, inputs)
			return nil
		}

		// Update Deckhouse-registry secret and wait
		processedRegistrySwitcher, err := state.processRegistrySwitcher(registrySecretParams, inputs)
		if err != nil {
			return err
		}
		if !processedRegistrySwitcher {
			state.setReadyCondition(false, inputs)
			return nil
		}

		state.Bashible.UnmanagedParams = nil
	}

	// Cleanup

	// Only input params. Or skip, if registry has never been enabled
	bashibleReady, err := state.processBashibleUnmanagedFinalize(inputs.RegistrySecret, inputs)
	if err != nil {
		return err
	}
	if !bashibleReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	nodeServicesReady, err := state.cleanupNodeServices(inputs)
	if err != nil {
		return fmt.Errorf("cannot cleanup NodeServices: %w", err)
	}
	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	state.RegistryService = registryservice.ModeDisabled

	state.IngressEnabled = false

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
	state.Bashible.UnmanagedParams = nil
	state.Mode = state.TargetMode
	state.setReadyCondition(true, inputs)

	return nil
}

func (state *State) bashiblePreflightCheck(inputs Inputs) bool {
	result := bashible.PreflightCheck(inputs.Bashible)

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashiblePreflightCheck,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashiblePreflightCheck,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return true
}

func (state *State) processBashibleTransition(params bashible.Params, inputs Inputs) (bool, error) {
	result, err := state.Bashible.ProcessTransition(params, inputs.Bashible)
	if err != nil {
		return false, fmt.Errorf("cannot process Bashible: %w", err)
	}

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashibleTransitionStage,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashibleTransitionStage,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) processBashibleFinalize(params bashible.Params, inputs Inputs) (bool, error) {
	result, err := state.Bashible.FinalizeTransition(params, inputs.Bashible)
	if err != nil {
		return false, fmt.Errorf("cannot process Bashible: %w", err)
	}

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashibleFinalStage,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashibleFinalStage,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) processBashibleUnmanagedFinalize(registrySecret deckhouse_registry.Config, inputs Inputs) (bool, error) {
	result, err := state.Bashible.FinalizeUnmanagedTransition(registrySecret, inputs.Bashible)
	if err != nil {
		return false, fmt.Errorf("cannot process Bashible: %w", err)
	}

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeBashibleFinalStage,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeBashibleFinalStage,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) processNodeServices(log go_hook.Logger, params nodeservices.Params, inputs Inputs) (nodeservices.ProcessResult, error) {
	result, err := state.NodeServices.Process(log, params, inputs.NodeServices)
	if err != nil {
		return result, fmt.Errorf("cannot process NodeServices: %w", err)
	}

	if !result.IsReady() {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeNodeServices,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.GetConditionMessage(),
		})
		return result, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServices,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return result, nil
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
			Type:               ConditionTypeNodeServicesCleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            builder.String(),
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeNodeServicesCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return true, nil
}

func (state *State) processInClusterProxy(log go_hook.Logger, params inclusterproxy.Params, inputs Inputs) (bool, error) {
	result, err := state.InClusterProxy.Process(log, params, inputs.InClusterProxy)
	if err != nil {
		return false, fmt.Errorf("cannot process InClusterProxy: %w", err)
	}

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeInClusterProxy,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeInClusterProxy,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) cleanupInClusterProxy(inputs Inputs) bool {
	result := state.InClusterProxy.Stop(inputs.InClusterProxy)

	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeInClusterProxyCleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeInClusterProxyCleanup,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})

	return true
}

func (state *State) processRegistrySwitcher(params registryswitcher.Params, inputs Inputs) (bool, error) {
	// Update Deckhouse-registry secret and wait
	result, err := state.RegistrySecret.Process(params, inputs.RegistrySwitcher)
	if err != nil {
		return false, err
	}
	if !result.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeDeckhouseRegistrySwitch,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            result.Message,
		})
		return false, nil
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeDeckhouseRegistrySwitch,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
	})
	return true, nil
}

func (state *State) processCheckerUpstream(params checker.RegistryParams, inputs Inputs) (bool, error) {
	checkerVersion, err := helpers.ComputeHash(
		params,
		state.TargetMode,
	)
	if err != nil {
		return false, fmt.Errorf("cannot compute checker params hash: %w", err)
	}

	registryAddr, _ := helpers.RegistryAddressAndPathFromImagesRepo(params.Address)
	state.CheckerParams = checker.Params{
		Registries: map[string]checker.RegistryParams{
			registryAddr: params,
		},
		Version: checkerVersion,
	}

	isReady := state.setCheckerCondition(inputs)
	return isReady, nil
}

func (state *State) processCheckerNodes(
	inputs Inputs,
	nodeServicesResult nodeservices.ProcessResult,
	user users.User,
	ca *x509.Certificate,
) (bool, error) {
	checkerRegistry := checker.RegistryParams{
		Scheme:   "HTTPS",
		Username: user.UserName,
		Password: user.Password,
	}

	if ca != nil {
		checkerRegistry.CA = string(registry_pki.EncodeCertificate(ca))
	}

	version, err := helpers.ComputeHash(
		checkerRegistry,
		state.TargetMode,
	)
	if err != nil {
		return false, fmt.Errorf("cannot compute checker params hash: %w", err)
	}

	state.CheckerParams = checker.Params{
		Registries: make(map[string]checker.RegistryParams),
		Version:    version,
	}

	for node, result := range nodeServicesResult {
		if result.Address == "" {
			continue
		}

		params := checkerRegistry
		params.Address = registry_const.NodeRegistryAddr(result.Address)
		state.CheckerParams.Registries[node] = params
	}

	isReady := state.setCheckerCondition(inputs)
	return isReady, nil
}

func (state *State) setCheckerCondition(inputs Inputs) bool {
	if inputs.CheckerStatus.Version != state.CheckerParams.Version {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeRegistryContainsRequiredImages,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            "Initializing",
		})

		return false
	}

	if !inputs.CheckerStatus.Ready {
		state.setCondition(metav1.Condition{
			Type:               ConditionTypeRegistryContainsRequiredImages,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: inputs.Params.Generation,
			Reason:             ConditionReasonProcessing,
			Message:            inputs.CheckerStatus.Message,
		})

		return false
	}

	state.setCondition(metav1.Condition{
		Type:               ConditionTypeRegistryContainsRequiredImages,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inputs.Params.Generation,
		Reason:             ConditionReasonReady,
		Message:            inputs.CheckerStatus.Message,
	})

	return true
}

func (state *State) cleanupUsupportedConditions() {
	conditions := make([]metav1.Condition, 0, len(state.Conditions))

	for _, c := range state.Conditions {
		if _, ok := supportedConditions[c.Type]; ok {
			conditions = append(conditions, c)
		}
	}

	state.Conditions = conditions
}

func (state *State) setReadyCondition(ready bool, inputs Inputs) {
	state.cleanupUsupportedConditions()

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

func encodeCertificateIfExist(cert *x509.Certificate) []byte {
	if cert != nil {
		return registry_pki.EncodeCertificate(cert)
	}
	return []byte{}
}
