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
	"context"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func moduleValidationHandler() http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(_ context.Context, review *model.AdmissionReview, _ metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		// UserInfo groups: [system:serviceaccounts system:serviceaccounts:d8-system system:authenticated]
		if review.UserInfo.Username != "system:serviceaccount:d8-system:deckhouse" {
			return rejectResult("manual Module change is forbidden")
		}

		return allowResult("")
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-operations",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.Module{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
