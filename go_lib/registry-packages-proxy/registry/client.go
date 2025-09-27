// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"io"

	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
)

const (
	// defaultRepository is the repository address from deckhouse-registry secret.
	DefaultRepository = "__default__"
)

var ErrPackageNotFound = errors.New("package not found")

type Client interface {
	GetPackage(ctx context.Context, log log.Logger, config *ClientConfig, digest string, path string) (int64, string, io.ReadCloser, error)
}
