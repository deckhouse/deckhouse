/*
Copyright 2026 Flant JSC

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// moduleV1alpha2ValidationHandler keeps a module read-only while a module config for it
// exists: the config owns the module spec and its reconciler syncs it on every reconcile,
// so a manual change would silently be overwritten. Without a config the module is the
// user's to manage.
//
// Creation is not restricted, so the webhook is registered for updates and deletions only.
func moduleV1alpha2ValidationHandler(cli client.Client) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		// the deckhouse controller owns the resource, it reconciles modules and keeps
		// their spec in sync with the module config
		if review.UserInfo.Username == deckhouseServiceAccount {
			return allowResult(nil)
		}

		module, ok := obj.(*v1alpha2.Module)
		if !ok {
			return nil, fmt.Errorf("expect Module as v1alpha2, got %T", obj)
		}

		// The module config and the module share their name. A lookup failure other than
		// not found is returned as an error, which the webhook serves as an HTTP 500 so
		// that the API server applies its failure policy: an unreadable module config
		// must not pass for an absent one.
		cfg := new(v1alpha1.ModuleConfig)
		if err := cli.Get(ctx, client.ObjectKey{Name: module.Name}, cfg); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("get the '%s' module config: %w", module.Name, err)
			}

			return allowResult(nil)
		}

		if review.Operation == model.OperationDelete {
			return rejectResult(fmt.Sprintf("manual Module deletion is forbidden, the '%s' module is managed by its ModuleConfig", module.Name))
		}

		return rejectResult(fmt.Sprintf("manual Module change is forbidden, the '%s' module is managed by its ModuleConfig, change it there instead", module.Name))
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-v1alpha2-operations",
		Validator: vf,
		// logger is nil, because webhook has Info level for reporting about http handler
		// and we get a log of useless spam here. So we decided to use Noop logger here
		Logger: nil,
		Obj:    &v1alpha2.Module{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
