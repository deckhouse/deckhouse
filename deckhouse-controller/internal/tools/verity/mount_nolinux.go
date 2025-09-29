//go:build !linux

package verity

import (
	"context"
)

// Mount is a no-op on non-Linux platforms to allow tests/builds to pass without erofs.
func Mount(ctx context.Context, module, mountPath string) error { //nolint:revive,unused
	return nil
}

// Unmount is a no-op on non-Linux platforms.
func Unmount(ctx context.Context, mountPath string) error { //nolint:revive,unused
	return nil
}

