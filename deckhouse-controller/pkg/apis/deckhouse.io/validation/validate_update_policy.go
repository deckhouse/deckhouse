/*
Copyright 2024 Flant JSC

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
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func updatePolicyHandler(cli client.Client) http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		policy, ok := obj.(*v1alpha2.ModuleUpdatePolicy)
		if !ok {
			log.Debug("unexpected type", log.Type("expected", v1alpha2.ModuleUpdatePolicy{}), log.Type("got", obj))

			return nil, fmt.Errorf("expect ModuleUpdatePolicy as unstructured, got %T", obj)
		}

		configs := new(v1alpha1.ModuleConfigList)
		if err := cli.List(context.Background(), configs); err != nil {
			return nil, fmt.Errorf("list configs: %w", err)
		}

		for _, config := range configs.Items {
			if config.Spec.UpdatePolicy == policy.Name {
				return rejectResult(fmt.Sprintf("the '%s' update policy is used by the '%s' module config", policy.Name, config.Name))
			}
		}

		return allowResult(nil)
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "update-policy-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1alpha2.ModuleUpdatePolicy{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
