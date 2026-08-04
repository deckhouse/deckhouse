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

package fill

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// CountCatalogue counts the distinct tags the registry actually holds.
//
// Only sound when the registry has no upstream to fall back on. With a
// pass-through cache this would count what the registry can FETCH rather than
// what it holds, which is precisely the mistake that would authorize dropping the
// upstream on an empty store. In an air-gapped cluster self-filling is impossible,
// so reading the catalogue is honest — and it is the only way to account for
// content that arrived out of band through `d8 mirror push`.
func CountCatalogue(ctx context.Context, registry Registry) (int32, error) {
	puller, err := remote.NewPuller(registry.Options...)
	if err != nil {
		return 0, fmt.Errorf("building the client: %w", err)
	}

	var options []name.Option
	if registry.Insecure {
		options = append(options, name.Insecure)
	}
	parsed, err := name.NewRegistry(registry.Address, options...)
	if err != nil {
		return 0, fmt.Errorf("parsing the registry address %q: %w", registry.Address, err)
	}

	catalogger, err := puller.Catalogger(ctx, parsed)
	if err != nil {
		return 0, fmt.Errorf("listing the repositories: %w", err)
	}

	var count int32

	for catalogger.HasNext() {
		page, err := catalogger.Next(ctx)
		if err != nil {
			return 0, fmt.Errorf("reading the next page of repositories: %w", err)
		}

		for _, repository := range page.Repos {
			if !inScope(registry.Repository, repository) {
				continue
			}

			lister, err := puller.Lister(ctx, parsed.Repo(repository))
			if err != nil {
				return 0, fmt.Errorf("listing the tags of %s: %w", repository, err)
			}
			for lister.HasNext() {
				tags, err := lister.Next(ctx)
				if err != nil {
					return 0, fmt.Errorf("reading the next page of tags of %s: %w", repository, err)
				}
				count += int32(len(tags.Tags))
			}
		}
	}

	return count, nil
}

func roundTripper(certificateAuthority string) (http.RoundTripper, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if certificateAuthority == "" {
		return transport, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM([]byte(certificateAuthority)) {
		return nil, errors.New("the configured certificate authority is not valid PEM")
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return transport, nil
}
