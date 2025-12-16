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

package orchestrator

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/incluster-proxy"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-service"
	registryswitcher "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-switcher"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/users"
)

type State struct {
	Mode       registry_const.ModeType `json:"mode,omitempty"`
	TargetMode registry_const.ModeType `json:"target_mode,omitempty"`

	PKI             pki.State              `json:"pki,omitempty"`
	Secrets         secrets.State          `json:"secrets,omitempty"`
	Users           users.State            `json:"users,omitempty"`
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

func (state *State) initialize(log go_hook.Logger, inputs Inputs) error {
	// Process PKI
	if inputs.InitSecret.CA != nil {
		state.PKI.CA = &pki.CertModel{
			Cert: inputs.InitSecret.CA.Cert,
			Key:  inputs.InitSecret.CA.Key,
		}
	}

	_, err := state.PKI.Process(log)
	if err != nil {
		return fmt.Errorf("cannot process PKI: %w", err)
	}

	// Set Bashible ActualParams
	var (
		bashibleActualParams    *bashible.ModeParams
		bashibleUnmanagedParams *bashible.UnmanagedModeParams
	)

	switch inputs.Params.Mode {
	case registry_const.ModeDirect:
		bashibleActualParams = &bashible.ModeParams{
			Direct: &bashible.DirectModeParams{
				ImagesRepo: inputs.Params.ImagesRepo,
				Scheme:     inputs.Params.Scheme,
				CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
				Username:   inputs.Params.UserName,
				Password:   inputs.Params.Password,
			},
		}

		bashibleUnmanagedParams = &bashible.UnmanagedModeParams{
			ImagesRepo: inputs.Params.ImagesRepo,
			Scheme:     inputs.Params.Scheme,
			CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
			Username:   inputs.Params.UserName,
			Password:   inputs.Params.Password,
		}

	case registry_const.ModeUnmanaged:
		// Only for configurable unmanaged mode
		if inputs.Params.ImagesRepo != "" {
			bashibleActualParams = &bashible.ModeParams{
				Unmanaged: &bashible.UnmanagedModeParams{
					ImagesRepo: inputs.Params.ImagesRepo,
					Scheme:     inputs.Params.Scheme,
					CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
					Username:   inputs.Params.UserName,
					Password:   inputs.Params.Password,
				},
			}

			bashibleUnmanagedParams = &bashible.UnmanagedModeParams{
				ImagesRepo: inputs.Params.ImagesRepo,
				Scheme:     inputs.Params.Scheme,
				CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
				Username:   inputs.Params.UserName,
				Password:   inputs.Params.Password,
			}
		}
	}

	state.Bashible.ActualParams = bashibleActualParams
	state.Bashible.UnmanagedParams = bashibleUnmanagedParams
	return nil
}

func (state *State) process(log go_hook.Logger, inputs Inputs) error {
	if inputs.Params.Mode == "" {
		inputs.Params.Mode = registry_const.ModeUnmanaged
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
	case registry_const.ModeDirect:
		return state.transitionToDirect(log, inputs)
	case registry_const.ModeUnmanaged:
		if inputs.Params.ImagesRepo != "" {
			return state.transitionToConfigurableUnmanaged(inputs)
		}
		return state.transitionToUnmanaged(inputs)
	default:
		return fmt.Errorf("unsupported mode: %v", state.TargetMode)
	}
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
		return fmt.Errorf("cannot process checker on upstream: %w", err)
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
	bashibleParams := bashible.Params{
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
	processedBashible, err := state.processBashibleTransition(bashibleParams, inputs)
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
	processedBashible, err = state.processBashibleFinalize(bashibleParams, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Cleanup
	state.IngressEnabled = false

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

func (state *State) transitionToConfigurableUnmanaged(inputs Inputs) error {
	// check upstream registry
	checkerRegistryParams := checker.RegistryParams{
		Address:  inputs.Params.ImagesRepo,
		Scheme:   strings.ToUpper(inputs.Params.Scheme),
		CA:       string(encodeCertificateIfExist(inputs.Params.CA)),
		Username: inputs.Params.UserName,
		Password: inputs.Params.Password,
	}

	checkerReady, err := state.processCheckerUpstream(checkerRegistryParams, inputs)
	if err != nil {
		return fmt.Errorf("cannot process checker on upstream: %w", err)
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

	bashibleParams := bashible.Params{
		RegistrySecret: inputs.RegistrySecret,
		ModeParams: bashible.ModeParams{
			Unmanaged: &bashible.UnmanagedModeParams{
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
		UnmanagedMode: &registryswitcher.UnmanagedModeParams{
			ImagesRepo: inputs.Params.ImagesRepo,
			Scheme:     inputs.Params.Scheme,
			CA:         string(encodeCertificateIfExist(inputs.Params.CA)),
			Username:   inputs.Params.UserName,
			Password:   inputs.Params.Password,
		},
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

	// Bashible only input params
	processedBashible, err = state.processBashibleFinalize(bashibleParams, inputs)
	if err != nil {
		return err
	}
	if !processedBashible {
		state.setReadyCondition(false, inputs)
		return nil
	}

	// Cleanup
	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	state.RegistryService = registryservice.ModeDisabled

	state.IngressEnabled = false

	if !inClusterProxyReady {
		state.setReadyCondition(false, inputs)
		return nil
	}

	state.PKI = pki.State{}
	state.Secrets = secrets.State{}
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

func (state *State) transitionToUnmanaged(inputs Inputs) error {
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

	if (state.Mode == registry_const.ModeUnmanaged ||
		state.Mode == registry_const.ModeProxy ||
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
			Password: unmanagedParams.Password,
		}

		checkerReady, err := state.processCheckerUpstream(checkerRegistryParams, inputs)
		if err != nil {
			return fmt.Errorf("cannot process checker on upstream: %w", err)
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

	inClusterProxyReady := state.cleanupInClusterProxy(inputs)

	state.RegistryService = registryservice.ModeDisabled

	state.IngressEnabled = false

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
	checkerVersion, err := registry_pki.ComputeHash(
		params,
		state.TargetMode,
		inputs.Params.CheckMode,
	)
	if err != nil {
		return false, fmt.Errorf("cannot compute checker params hash: %w", err)
	}

	registryAddr, _ := registry_helpers.SplitAddressAndPath(params.Address)
	state.CheckerParams = checker.Params{
		Registries: map[string]checker.RegistryParams{
			registryAddr: params,
		},
		CheckMode: inputs.Params.CheckMode,
		Version:   checkerVersion,
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
