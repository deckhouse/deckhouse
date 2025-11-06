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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RegistrySecretHandler() http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		secret, ok := obj.(*v1.Secret)
		if !ok {
			return nil, fmt.Errorf("expected Secret object, got %T", obj)
		}

		if secret.Namespace != "d8-system" || secret.Name != "deckhouse-registry" {
			return &kwhvalidating.ValidatorResult{Valid: true}, nil
		}

		requiredFields := []string{"address", "path", ".dockerconfigjson"}
		for _, field := range requiredFields {
			if _, ok := secret.Data[field]; !ok {
				return rejectRes(fmt.Sprintf("Field '%s' is required in deckhouse-registry secret.", field))
			}
		}

		for _, field := range []string{"address", "path"} {
			val := string(secret.Data[field])
			if strings.TrimSpace(val) == "" {
				return rejectRes(fmt.Sprintf("Field '%s' cannot be empty or whitespace.", field))
			}
			if containsWhitespace(val) {
				return rejectRes(fmt.Sprintf("Field '%s' contains spaces or newlines.", field))
			}
		}

		dockerCfgRaw := secret.Data[".dockerconfigjson"]
		var dockerCfg struct {
			Auths map[string]struct {
				Auth string `json:"auth"`
			} `json:"auths"`
		}

		if err := json.Unmarshal(dockerCfgRaw, &dockerCfg); err != nil {
			return rejectResult(".dockerconfigjson is not valid JSON.")
		}

		if len(dockerCfg.Auths) == 0 {
			return rejectResult(".dockerconfigjson must contain at least one registry in 'auths'.")
		}

		for registry, authObj := range dockerCfg.Auths {
			if containsWhitespace(registry) {
				return rejectResult(fmt.Sprintf("Registry key '%s' contains spaces or newlines.", registry))
			}

			if authObj.Auth != "" {
				decodedAuth, err := base64.StdEncoding.DecodeString(authObj.Auth)
				if err != nil {
					return rejectResult(fmt.Sprintf("Credentials for registry '%s' are not valid base64.", registry))
				}

				parts := strings.SplitN(string(decodedAuth), ":", 2)
				if len(parts) != 2 {
					return rejectResult(fmt.Sprintf("Credentials for registry '%s' must be in format login:password.", registry))
				}

				login := parts[0]
				password := parts[1]

				if login == "" || containsWhitespace(password) {
					return rejectResult(fmt.Sprintf("Login for registry '%s' contains spaces, tabs, newlines or empty.", registry))
				}

				if password != "" && containsWhitespace(password) {
					return rejectResult(fmt.Sprintf("Password for registry '%s' contains spaces, tabs, or newlines.", registry))
				}
			}
		}

		return &kwhvalidating.ValidatorResult{Valid: true}, nil
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "deckhouse-registry-secret-validator",
		Validator: validator,
		Obj:       &v1.Secret{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh})
}

func rejectRes(msg string) (*kwhvalidating.ValidatorResult, error) {
	return &kwhvalidating.ValidatorResult{
		Valid:   false,
		Message: msg,
	}, nil
}

func containsWhitespace(s string) bool {
	whitespaceRe := regexp.MustCompile(`[\s\r\n\t]`)
	return whitespaceRe.MatchString(s)
}
