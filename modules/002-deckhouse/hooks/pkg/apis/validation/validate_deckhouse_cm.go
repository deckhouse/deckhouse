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
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func deckhouseCMValidationHandler() http.Handler {
	vf := kwhvalidating.ValidatorFunc(func(ctx context.Context, review *model.AdmissionReview, obj metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		cmName := obj.GetName()

		if cmName != os.Getenv("ADDON_OPERATOR_CONFIG_MAP") {
			return allowResult("")
		}

		operation := "changing"
		if review.Operation == kwhmodel.OperationDelete {
			operation = "deleting"
		}

		if review.UserInfo.Username == "system:serviceaccount:d8-system:deckhouse" || review.UserInfo.Username == "system:serviceaccount:kube-system:generic-garbage-collector" {
			return allowResult("")
		}

		log.Infof("Request to %s ConfigMap/%s by user %+v", string(review.Operation), cmName, review.UserInfo)

		return rejectResult(fmt.Sprintf("%s ConfigMap/%s is not allowed for %s. Use ModuleConfig resources to configure Deckhouse.", operation, cmName, review.UserInfo.Username))
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "deckhouse-cm-operations",
		Validator: vf,
		Logger:    validationLogger,
		Obj:       &v1.ConfigMap{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: validationLogger})
}
