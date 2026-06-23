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

package cache

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstrumentedClient struct {
	client.Client
}

func (c *InstrumentedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	cacheRequestsTotal.WithLabelValues("get").Inc()
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *InstrumentedClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	cacheRequestsTotal.WithLabelValues("list").Inc()
	return c.Client.List(ctx, list, opts...)
}
