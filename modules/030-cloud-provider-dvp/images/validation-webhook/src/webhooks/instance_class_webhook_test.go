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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDVPInstanceClassValidatorWithFakeClientValidateUpdate(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(builder, &unstructured.Unstructured{})

	updated := dvpInstanceClassObject("master-dvp")
	updated.Object["spec"] = map[string]any{"etcdDisk": map[string]any{"size": int64(10)}}
	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestDVPInstanceClassValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(builder, &unstructured.Unstructured{})

	created := dvpInstanceClassObject("worker-dvp")
	_, err := validator.ValidateCreate(context.Background(), created)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestDVPInstanceClassValidatorWithFakeClientRejectsDeleteInUse(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), dvpInstanceClassObject("master-dvp"))
	if err == nil || !strings.Contains(err.Error(), "InstanceClass is used by NodeGroup") {
		t.Fatalf("ValidateDelete() error = %v, want in-use denial", err)
	}
}
