package proxy

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/distribution/registry/storage/driver"
)

func CleanupCacheStorage(ctx context.Context, storage driver.StorageDriver) error {
	exists, err := scheduleStateExists(ctx, storage)
	if err != nil {
		return err
	}
	if exists {
		return cleanupStorage(ctx, storage)
	}
	return nil
}

func cleanupNonCacheStorage(ctx context.Context, storage driver.StorageDriver) error {
	exists, err := scheduleStateExists(ctx, storage)
	if err != nil {
		return err
	}
	if !exists {
		return cleanupStorage(ctx, storage)
	}
	return nil
}

func cleanupStorage(ctx context.Context, storage driver.StorageDriver) error {
	if err := pathDelete(ctx, storage, schedulerStateFilePath); err != nil {
		return err
	}
	// registry/storage/paths.go:13
	if err := pathDelete(ctx, storage, "/docker"); err != nil {
		return err
	}
	return nil
}

func scheduleStateExists(ctx context.Context, storage driver.StorageDriver) (bool, error) {
	return pathExists(ctx, storage, schedulerStateFilePath)
}

func pathExists(ctx context.Context, storage driver.StorageDriver, path string) (bool, error) {
	_, err := storage.Stat(ctx, path)
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &driver.PathNotFoundError{}):
		return false, nil
	default:
		return false, fmt.Errorf("stat %q: %w", path, err)
	}
}

func pathDelete(ctx context.Context, storage driver.StorageDriver, path string) error {
	err := storage.Delete(ctx, path)
	switch {
	case err == nil:
		return nil
	case errors.As(err, &driver.PathNotFoundError{}):
		return nil
	default:
		return fmt.Errorf("delete %q: %w", path, err)
	}
}
