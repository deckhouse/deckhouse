// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestModuleConfigValidatorWithFakeClientValidateCreate(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewModuleConfigValidator(builder, &unstructured.Unstructured{})

	obj := dvpModuleConfigObject()
	obj.Object["spec"] = map[string]any{"enabled": true, "version": int64(2)}

	_, err := validator.ValidateCreate(context.Background(), obj)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestModuleConfigValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewModuleConfigValidator(builder, &unstructured.Unstructured{})

	obj := dvpModuleConfigObject()
	obj.Object["spec"] = map[string]any{"enabled": true, "version": int64(2)}

	_, err := validator.ValidateUpdate(context.Background(), nil, obj)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestModuleConfigValidatorWithFakeClientIgnoresOtherModules(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t)
	validator := NewModuleConfigValidator(builder, &unstructured.Unstructured{})

	obj := dvpModuleConfigObject()
	obj.SetName("other-module")

	_, err := validator.ValidateUpdate(context.Background(), nil, obj)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow for unrelated module", err)
	}
}

func TestModuleConfigValidatorWithFakeClientAllowsIncompleteStack(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, dvpModuleConfigObject())
	validator := NewModuleConfigValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateUpdate(context.Background(), nil, dvpModuleConfigObject())
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow without preflight requirements", err)
	}
}

func TestModuleConfigValidatorWithFakeClientValidateDeleteAllowed(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t)
	validator := NewModuleConfigValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), dvpModuleConfigObject())
	if err != nil {
		t.Fatalf("ValidateDelete() error = %v, want allow", err)
	}
}
