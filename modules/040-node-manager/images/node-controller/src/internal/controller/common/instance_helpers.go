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

package common

import (
	"context"
	"fmt"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureInstanceExists(
	ctx context.Context,
	c client.Client,
	name string,
	spec deckhousev1alpha2.InstanceSpec,
) (*deckhousev1alpha2.Instance, error) {
	instance := &deckhousev1alpha2.Instance{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err == nil {
		return instance, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get instance %q: %w", name, err)
	}

	newInstance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	if err := c.Create(ctx, newInstance); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err != nil {
				return nil, fmt.Errorf("get instance %q after already exists: %w", name, err)
			}
			return instance, nil
		}

		return nil, fmt.Errorf("create instance %q: %w", name, err)
	}

	return newInstance, nil
}
