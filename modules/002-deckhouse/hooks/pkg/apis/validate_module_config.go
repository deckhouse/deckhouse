package apis

import (
	"context"
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	d8v1alpha1 "github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis/v1alpha1"
)

func moduleConfigValidationHandler() http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		switch review.Operation {
		case kwhmodel.OperationDelete:
			// Always allow deletion.
			return allowResult("")

		case kwhmodel.OperationConnect, kwhmodel.OperationUnknown:
			return rejectResult(fmt.Sprintf("operation '%s' is not applicable", review.Operation))
		}

		cfg, ok := obj.(*d8v1alpha1.ModuleConfig)
		if !ok {
			return nil, fmt.Errorf("expect ModuleConfig as unstructured, got %T", obj)
		}

		// Allow changing configuration for unknown modules.
		if !d8config.Service().PossibleNames().Has(cfg.Name) {
			return allowResult(fmt.Sprintf("module name '%s' is unknown for deckhouse", cfg.Name))
		}

		// Check if spec.version value is valid and the version is the latest.
		// Validate spec.settings using the OpenAPI schema.
		res := d8config.Service().ConfigValidator().Validate(cfg)
		if res.HasError() {
			return rejectResult(res.Error)
		}

		// Return allow with warning.
		return allowResult(res.Warning)
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-config-operations",
		Validator: vf,
		Logger:    validationLogger,
		Obj:       &d8v1alpha1.ModuleConfig{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: validationLogger})
}
