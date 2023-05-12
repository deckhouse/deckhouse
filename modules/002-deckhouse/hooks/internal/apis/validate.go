package apis

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/internal/apis/v1alpha1"
)

func init() {
	module.RegisterValidationHandler("/validate/v1alpha1/module", moduleValidationHandler())
}

func moduleValidationHandler() http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		// UserInfo groups: [system:serviceaccounts system:serviceaccounts:d8-system system:authenticated]
		if review.UserInfo.Username != "system:serviceaccount:d8-system:deckhouse" {
			return &kwhvalidating.ValidatorResult{
				Valid:   false,
				Message: "manual Module change is forbidden",
			}, nil
		}

		return &kwhvalidating.ValidatorResult{
			Valid:   true,
			Message: "",
		}, nil
	})

	kl := kwhlogrus.NewLogrus(log.NewEntry(log.StandardLogger()))

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "module-operations",
		Validator: vf,
		Logger:    kl,
		Obj:       &v1alpha1.Module{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: kl})
}
