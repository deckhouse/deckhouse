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

package hooks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func Test_cleanGarbageDeletesVirtualMachineBlockDeviceAttachment(t *testing.T) {
	ctx := context.Background()
	client := fakeK8sClient{
		Interface: kubernetesfake.NewSimpleClientset(),
		dynamic: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "virtualization.deckhouse.io/v1alpha2",
				"kind":       "VirtualMachineBlockDeviceAttachment",
				"metadata": map[string]interface{}{
					"name":              "probe-extra-disk-attachment",
					"namespace":         "upmeter-vm-lifecycle-agent",
					"creationTimestamp": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
					"labels": map[string]interface{}{
						"heritage": "upmeter",
					},
				},
			},
		}),
	}

	err := cleanGarbage(ctx, &virtualMachineBlockDeviceAttachmentRepo{k: client})
	assert.NoError(t, err)

	_, err = client.Dynamic().
		Resource(virtualMachineBlockDeviceAttachmentGVR).
		Namespace("upmeter-vm-lifecycle-agent").
		Get(ctx, "probe-extra-disk-attachment", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "VirtualMachineBlockDeviceAttachment should be deleted")
}

type fakeK8sClient struct {
	kubernetes.Interface
	dynamic dynamic.Interface
}

func (c fakeK8sClient) Dynamic() dynamic.Interface {
	return c.dynamic
}
