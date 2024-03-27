package app

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"

	"registry-packages-proxy/internal/cache"
)

func NewCache(ctx context.Context) (*cache.Cache, error) {
	if DisableCache {
		return nil, nil
	}

	quantity, err := resource.ParseQuantity(CacheRetentionSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse cache retention size")
	}

	cache, err := cache.New(CacheDirectory, uint64(quantity.Value()), CacheRetentionPeriod, cacheMetrics)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cache")
	}

	go func() {
		err := cache.Run(ctx)
		if err != nil {
			log.Errorf("Run cache: %v", err)
		}
	}()

	return cache, nil
}
