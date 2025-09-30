//go:build !linux

package verity

import (
	"context"
)

// CreateMapper is a no-op on non-Linux platforms.
func CreateMapper(ctx context.Context, imagePath, hash string) error { //nolint:revive,unused
	return nil
}

// CloseMapper is a no-op on non-Linux platforms.
func CloseMapper(ctx context.Context, module string) error { //nolint:revive,unused
	return nil
}

// CreateImageHash returns a deterministic fake hash on non-Linux platforms.
func CreateImageHash(ctx context.Context, imagePath string) (string, error) { //nolint:revive,unused
	return "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil
}

// VerifyImage is a no-op on non-Linux platforms.
func VerifyImage(ctx context.Context, imagePath, rootHash string) error { //nolint:revive,unused
	return nil
}
