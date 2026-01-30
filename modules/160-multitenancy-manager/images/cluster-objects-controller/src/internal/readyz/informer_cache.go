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
		grantInformer, err := c.GetInformer(ctx, &v1alpha1.ClusterObjectsGrant{})
		if err != nil {
			return fmt.Errorf("get grants informer: %w", err)
		}

		policyInformer, err := c.GetInformer(ctx, &v1alpha1.ClusterObjectGrantPolicy{})
		if err != nil {
			return fmt.Errorf("get policies informer: %w", err)
		}

		if grantInformer.HasSynced() && policyInformer.HasSynced() {
			return nil
		}

		return errors.New("waiting for informer caches to sync")
	}
}
