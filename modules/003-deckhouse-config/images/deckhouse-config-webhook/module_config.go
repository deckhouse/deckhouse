/*
Copyright 2022 Flant JSC

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

package main

import (
	"context"
	"fmt"

	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
)

type ModuleConfigValidator struct {
	modulesDir     string
	globalHooksDir string
	modulesMap     map[string]struct{}
}

func NewModuleConfigValidator(globalHooksDir string, modulesDir string) *ModuleConfigValidator {
	return &ModuleConfigValidator{
		globalHooksDir: globalHooksDir,
		modulesDir:     modulesDir,
	}
}

func (c *ModuleConfigValidator) Validate(_ context.Context, review *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	switch review.Operation {
	case kwhmodel.OperationDelete:
		// Always allow deletion.
		return allowResult("")
	case kwhmodel.OperationConnect:
		fallthrough
	case kwhmodel.OperationUnknown:
		return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))
	}

	cfg, err := getModuleConfig(obj)
	if err != nil {
		return nil, err
	}

	// Allow changing configuration for unknown modules.
	if !d8config.Service().PossibleNames().Has(cfg.Name) {
		return allowResult(fmt.Sprintf("module name '%s' is unknown for deckhouse", cfg.Name))
	}

	// Check if spec.version value is valid and the version is the latest.
	// Validate spec.settings using the OpenAPI schema.
	res, err := d8config.Service().ConfigValidator().Validate(cfg)
	if err != nil {
		return rejectResult(err.Error())
	}

	// Return allow with warning about the latest version.
	return allowResult(res.VersionWarning)
}

func getModuleConfig(obj metav1.Object) (*d8cfg_v1alpha1.ModuleConfig, error) {
	untypedCfg, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
	}

	if untypedCfg.GetKind() != "ModuleConfig" {
		return nil, fmt.Errorf("expect ModuleConfig, got %s", untypedCfg.GetKind())
	}

	var cfg d8cfg_v1alpha1.ModuleConfig
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(untypedCfg.UnstructuredContent(), &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func allowResult(warnMsg string) (*kwhvalidating.ValidatorResult, error) {
	var warnings []string
	if warnMsg != "" {
		warnings = []string{warnMsg}
	}
	return &kwhvalidating.ValidatorResult{
		Valid:    true,
		Warnings: warnings,
	}, nil
}

func rejectResult(msg string) (*kwhvalidating.ValidatorResult, error) {
	return &kwhvalidating.ValidatorResult{
		Valid:   false,
		Message: msg,
	}, nil
}
