//go:build !linux

package verity

import (
	"context"
	"io"
)

// CreateImage is a no-op on non-Linux platforms.
func CreateImage(ctx context.Context, modulePath, imagePath string) error { //nolint:revive,unused
	return nil
}

// CreateImageByTar is a no-op on non-Linux platforms.
func CreateImageByTar(ctx context.Context, rc io.ReadCloser, imagePath string) error { //nolint:revive,unused
	return nil
}

