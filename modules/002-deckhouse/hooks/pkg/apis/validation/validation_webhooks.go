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

package validation

import (
	log "github.com/sirupsen/logrus"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

var (
	validationLogger = kwhlogrus.NewLogrus(log.NewEntry(log.StandardLogger()))
)

func init() {
	module.RegisterValidationHandler("/validate/v1alpha1/modules", moduleValidationHandler())
	module.RegisterValidationHandler("/validate/v1alpha1/module-configs", moduleConfigValidationHandler())
	module.RegisterValidationHandler("/validate/core/v1/configmap", deckhouseCMValidationHandler())
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
