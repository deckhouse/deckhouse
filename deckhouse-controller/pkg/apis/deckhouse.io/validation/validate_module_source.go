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
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func sourceValidationHandler(cli client.Client) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		source, ok := obj.(*v1alpha1.ModuleSource)
		if !ok {
			log.Debug("unexpected type", log.Type("expected", v1alpha1.ModuleSource{}), log.Type("got", obj))

			return nil, fmt.Errorf("expect ModuleSource as unstructured, got %T", obj)
		}

		if source.IsDefault() {
			sources := new(v1alpha1.ModuleSourceList)
			if err := cli.List(context.Background(), sources); err != nil {
				return nil, fmt.Errorf("list sources: %w", err)
			}

			for _, item := range sources.Items {
				if item.Name != source.Name && item.IsDefault() {
					return rejectResult("there can only be one default source")
				}
			}
		}

		return allowResult(nil)
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "source-operations",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha1.ModuleSource{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
