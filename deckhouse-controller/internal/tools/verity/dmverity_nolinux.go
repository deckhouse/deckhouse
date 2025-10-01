//go:build !linux

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
