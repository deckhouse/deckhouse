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

package api

import (
	"context"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	clientset *kubernetes.Clientset
	client    client.Client
	namespace string
}

const defaultWaitCheckInterval = time.Second

type WaitFn func(obj client.Object) (bool, error)

func (c *Service) Wait(ctx context.Context, name string, obj client.Object, waitFn WaitFn) error {
	var done bool
	for {
		err := c.client.Get(ctx, types.NamespacedName{
			Namespace: c.namespace,
			Name:      name,
		}, obj)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				return err
			}

			// obj not found.
			done, err = waitFn(nil)
		} else {
			// obj found.
			done, err = waitFn(obj)
		}

		if err != nil {
			return err
		}

		if done {
			return nil
		}

		timer := time.NewTimer(defaultWaitCheckInterval)

		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}
