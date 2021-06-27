// Copyright 2021 Flant CJSC
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

package docker_registry_watcher

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
)

func ImageDigest(ref name.Reference) (string, error) {
	img, err := GetImage(ref)
	if err != nil {
		return "", err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", err
	}

	return digest.String(), nil
}

func GetImage(ref name.Reference) (v1.Image, error) {

	img, err := remote.Image(ref,
		remote.WithAuthFromKeychain(NewKeychain()),
		remote.WithTransport(GetHTTPTransport()))

	if err != nil {
		return nil, fmt.Errorf("reading image %q: %v", ref, err)
	}

	return img, nil
}

func ParseReferenceOptions() []name.Option {
	var options []name.Option
	options = append(options, name.WeakValidation)

	if app.InsecureRegistry == "yes" {
		options = append(options, name.Insecure)
	}

	return options
}

func GetHTTPTransport() (transport http.RoundTripper) {
	if app.SkipTLSVerifyRegistry == "yes" {
		// default http transport with InsecureSkipVerify
		return &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		}
	}
	return http.DefaultTransport
}
