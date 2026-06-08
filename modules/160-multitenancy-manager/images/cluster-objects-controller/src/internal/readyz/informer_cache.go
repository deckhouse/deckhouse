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

package readyz

import (
	"errors"
	"fmt"
	"net/http"

	"controller/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// ProbeWithInformerCacheStatus returns a probe that checks if all informer caches are synced before responding ready status
func ProbeWithInformerCacheStatus(c cache.Cache) func(*http.Request) error {
	return func(req *http.Request) error {
		ctx := req.Context()
		grantInformer, err := c.GetInformer(ctx, &v1alpha1.ClusterObjectGrant{})
		if err != nil {
			return fmt.Errorf("get grants informer: %w", err)
		}

		regInformer, err := c.GetInformer(ctx, &v1alpha1.ClusterGrantableResource{})
		if err != nil {
			return fmt.Errorf("get grantable resources informer: %w", err)
		}

		if grantInformer.HasSynced() && regInformer.HasSynced() {
			return nil
		}

		return errors.New("waiting for informer caches to sync")
	}
}