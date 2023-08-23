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

package util

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get uses the client and reference to get an unstructured object.
func Get(ctx context.Context, reader client.Reader, ref *corev1.ObjectReference, namespace string) (*unstructured.Unstructured, error) {
	if ref == nil {
		return nil, errors.Errorf("cannot get object - object reference not set")
	}

	obj := new(unstructured.Unstructured)
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	key := client.ObjectKey{Name: obj.GetName(), Namespace: namespace}

	err := reader.Get(ctx, key, obj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve %s object %q/%q", obj.GetKind(), key.Namespace, key.Name)
	}

	return obj, nil
}
