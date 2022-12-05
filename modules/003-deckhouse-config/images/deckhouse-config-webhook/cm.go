/*
Copyright 2022 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapValidator struct {
	eligibleNames map[string]struct{}
	allowedUsers  map[string]struct{}
}

func NewConfigMapValidator(names string, users string) *ConfigMapValidator {
	v := &ConfigMapValidator{
		eligibleNames: make(map[string]struct{}),
		allowedUsers:  make(map[string]struct{}),
	}
	namesList := strings.Split(names, ",")
	for _, name := range namesList {
		v.eligibleNames[name] = struct{}{}
	}
	usersList := strings.Split(users, ",")
	for _, user := range usersList {
		v.allowedUsers[user] = struct{}{}
	}
	return v
}

func (c *ConfigMapValidator) Validate(_ context.Context, review *kwhmodel.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	cmName := review.Name

	_, isEligibleName := c.eligibleNames[cmName]
	_, isAllowedUser := c.allowedUsers[review.UserInfo.Username]

	if isEligibleName && !isAllowedUser {
		operation := "changing"
		if review.Operation == kwhmodel.OperationDelete {
			operation = "deleting"
		}
		log.Infof("Request to %s ConfigMap/%s by user %+v", string(review.Operation), cmName, review.UserInfo)
		return &kwhvalidating.ValidatorResult{
			Valid:   false,
			Message: fmt.Sprintf("%s ConfigMap/%s is not allowed for %s. Use ModuleConfig resources to configure Deckhouse.", operation, cmName, review.UserInfo.Username),
		}, nil
	}

	return &kwhvalidating.ValidatorResult{
		Valid: true,
	}, nil
}
