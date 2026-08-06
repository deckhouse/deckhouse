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

// moduleV1alpha2ValidationHandler restricts manual changes to v1alpha2 Modules:
// they are created by the deckhouse controller, and while a module config for the
// module exists that config owns the module spec — the config reconciler syncs it
// on every reconcile, so a manual edit would silently be overwritten.
//
// Deletion is forbidden as it always has been for the v1alpha1 version of the
// resource: deleting a module uninstalls it, and the module config is what a user
// removes to get rid of a module.
func moduleV1alpha2ValidationHandler(cli client.Client) http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		// the deckhouse controller owns the resource, it both creates modules and
		// keeps their spec in sync with the module config
		if review.UserInfo.Username == deckhouseServiceAccount {
			return allowResult(nil)
		}

		module, ok := obj.(*v1alpha2.Module)
		if !ok {
			return nil, fmt.Errorf("expect Module as v1alpha2, got %T", obj)
		}

		switch review.Operation {
		case model.OperationCreate:
			return rejectResult("manual Module creation is forbidden")

		case model.OperationUpdate:
			return validateModuleUpdate(ctx, cli, module.Name)

		case model.OperationDelete:
			return rejectResult("manual Module deletion is forbidden")

		default:
			return allowResult(nil)
		}
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

// validateModuleUpdate rejects the change when a module config with the module
// name exists — the module config and the module share their name.
//
// A lookup failure other than not found is returned as an error, which the
// webhook serves as an HTTP 500 so that the API server applies its failure
// policy: an unreadable module config must not pass for an absent one.
func validateModuleUpdate(ctx context.Context, cli client.Client, moduleName string) (*kwhvalidating.ValidatorResult, error) {
	cfg := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: moduleName}, cfg); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module config: %w", moduleName, err)
		}

		return allowResult(nil)
	}

	return rejectResult(fmt.Sprintf("manual Module change is forbidden, the '%s' module is managed by its module config", moduleName))
}
