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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNodeGroupValidatorWithFakeClientAllowsMasterCreateBeforeInstanceClass(t *testing.T) {
	t.Parallel()

	objects := []runtime.Object{
		dvpModuleConfigObject(),
		dvpCredentialSecret(validWebhookKubeconfigB64()),
	}
	builder := newWebhookAdmissionStateBuilder(t, objects...)
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow master NodeGroup before InstanceClass exists", err)
	}
}

func TestNodeGroupValidatorWithFakeClientValidateUpdate(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	updated := dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), dvpNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsStaticNodeGroupWhenStackIncomplete(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, dvpModuleConfigObject())
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), dvpStaticNodeGroupObject("worker-static"))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow for Static NodeGroup", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsMasterDemotion(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	oldMaster := dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	newMaster := dvpStaticNodeGroupObject("master")

	_, err := validator.ValidateUpdate(context.Background(), oldMaster, newMaster)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow without preflight requirements", err)
	}
}

func TestNodeGroupValidatorWithFakeClientValidateDeleteAllowsMaster(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewNodeGroupValidator(builder, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("ValidateDelete() error = %v, want allow master deletion without preflight", err)
	}
}

func TestShouldValidateNodeGroup(t *testing.T) {
	t.Parallel()

	if !shouldValidateNodeGroup(dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)) {
		t.Fatal("shouldValidateNodeGroup(CloudPermanent) = false, want true")
	}

	if shouldValidateNodeGroup(dvpStaticNodeGroupObject("worker-static")) {
		t.Fatal("shouldValidateNodeGroup(Static) = true, want false")
	}

	oldMaster := dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	newStatic := dvpStaticNodeGroupObject("master")
	if !shouldValidateNodeGroupUpdate(oldMaster, newStatic) {
		t.Fatal("shouldValidateNodeGroupUpdate(master demotion) = false, want true")
	}
}
