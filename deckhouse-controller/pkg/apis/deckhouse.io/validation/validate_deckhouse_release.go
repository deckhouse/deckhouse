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
)

type deckhouseReleaseModuleManager interface {
	GetEnabledModuleNames() []string
}

// deckhouseReleaseValidationHandler creates a webhook handler for DeckhouseRelease validation
func deckhouseReleaseValidationHandler(
	cli client.Client,
	moduleManager deckhouseReleaseModuleManager,
	exts *extenders.ExtendersStack,
) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		dr, ok := obj.(*v1alpha1.DeckhouseRelease)
		if !ok {
			return nil, fmt.Errorf("expect DeckhouseRelease as unstructured, got %T", obj)
		}

		if !dr.Approved {
			return allowResult(nil)
		}

		if review.Operation == model.OperationUpdate {
			if review.OldObjectRaw != nil {
				oldDR := &v1alpha1.DeckhouseRelease{}
				if err := json.Unmarshal(review.OldObjectRaw, oldDR); err == nil {
					if oldDR.Approved {
						return allowResult(nil)
					}
				}
			}
		}

		checker, err := releaseUpdater.NewDeckhouseReleaseRequirementsChecker(
			cli,
			moduleManager.GetEnabledModuleNames(),
			exts,
		)
		if err != nil {
			return rejectResult(fmt.Sprintf("failed to create requirements checker: %v", err))
		}

		reasons := checker.MetRequirements(ctx, dr)
		if len(reasons) > 0 {
			msgs := make([]string, 0, len(reasons))
			for _, reason := range reasons {
				msgs = append(msgs, reason.Message)
			}

			message := fmt.Sprintf("Cannot approve DeckhouseRelease %q: requirements not met: %s", dr.Name, strings.Join(msgs, "; "))
			return rejectResult(message)
		}

		return allowResult(nil)
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "deckhouse-release-approval",
		Validator: vf,
		Logger:    nil,
		Obj:       &v1alpha1.DeckhouseRelease{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
