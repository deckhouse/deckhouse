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

package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type deckhouseReleaseModuleManager interface {
	GetEnabledModuleNames() []string
}

// DeckhouseReleaseValidationHandler creates a webhook handler for DeckhouseRelease validation
func DeckhouseReleaseValidationHandler(
	client client.Client,
	metricStorage metricsstorage.Storage,
	moduleManager deckhouseReleaseModuleManager,
	exts *extenders.ExtendersStack,
) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		return validateDeckhouseReleaseApproval(ctx, review, obj, client, metricStorage, moduleManager, exts)
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "deckhouse-release-approval",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.DeckhouseRelease{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}

// validateDeckhouseReleaseApproval performs the main validation logic for DeckhouseRelease approval webhook
func validateDeckhouseReleaseApproval(
	ctx context.Context,
	review *model.AdmissionReview,
	obj metav1.Object,
	client client.Client,
	metricStorage metricsstorage.Storage,
	moduleManager deckhouseReleaseModuleManager,
	exts *extenders.ExtendersStack,
) (*kwhvalidating.ValidatorResult, error) {
	dr, ok := obj.(*v1alpha1.DeckhouseRelease)
	if !ok {
		return nil, fmt.Errorf("expect DeckhouseRelease as unstructured, got %T", obj)
	}

	// If the DeckhouseRelease is not approved, allow it
	if !dr.GetManuallyApproved() {
		return allowResult(nil)
	}

	if review.Operation == model.OperationUpdate {
		if review.OldObjectRaw != nil {
			oldDR := &v1alpha1.DeckhouseRelease{}
			if err := json.Unmarshal(review.OldObjectRaw, oldDR); err == nil {
				// If the old DeckhouseRelease was approved, we allow the update.
				// This is to prevent the case when a user approves a DeckhouseRelease,
				// but then tries to change it back to unapproved, or change another fields.
				if oldDR.Approved {
					return allowResult(nil)
				}
			}
		}
	}

	checker, err := releaseUpdater.NewDeckhouseReleaseRequirementsChecker(
		client,
		moduleManager.GetEnabledModuleNames(),
		exts,
		metricStorage,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create requirements checker: %v", err)
	}

	reasons := checker.MetRequirements(ctx, dr)
	if len(reasons) > 0 {
		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		message := fmt.Sprintf("\n cannot approve DeckhouseRelease %q: requirements not met: \n- %s", dr.Name, strings.Join(msgs, "\n- "))

		return rejectResult(message)
	}

	return allowResult(nil)
}
