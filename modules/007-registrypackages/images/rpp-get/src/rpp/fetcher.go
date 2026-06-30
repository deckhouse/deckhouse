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

package rpp

import (
	"context"
	"io"
)

// fetcher retrieves a package archive (a gzipped tar of the image's last layer)
// for a given OCI manifest digest. source is a human-readable origin (host) used
// only for logging.
type fetcher interface {
	Get(ctx context.Context, digest string) (io.ReadCloser, string, error)
}

func newFetcher(cfg Config) fetcher {
	if cfg.RegistryDirect {
		return newDirectClient(cfg)
	}
	return newHTTPClient(cfg)
}
