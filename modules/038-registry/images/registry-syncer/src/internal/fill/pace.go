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
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/time/rate"
)

// paceBurst is the most a single read takes from the pacer at once. Small enough that pacing is
// smooth at a few MiB/s, large enough that the limiter is not consulted per packet.
const paceBurst = 256 << 10

// Pacer bounds how fast the copy reads from its source, across every copy the process runs.
//
// A copy nobody is waiting for is background work on a control-plane node, and nothing else
// bounded it: every byte costs TLS on both sides, a digest on both sides and a write with an
// fsync, so the copy takes as much CPU and disk as the link to its source allows. Measured on
// DVP and Yandex masters, about 0.02-0.05 of a core per MiB/s — a fast link puts several cores
// and the whole disk queue under the copy, beside etcd and the API server, while the platform
// is starting up. Pacing the source bounds all of it at once: what is not read is not hashed,
// not re-encrypted and not written.
//
// Pulls by the nodes do not go through a Pacer: they are what the cache is for, and a node
// waiting for an image is not background work. Nor does copying the set the cluster runs — the
// leader's fill and the followers' replication — for the same reason one step removed: until a store
// holds the set, the nodes' pulls go past it, and a follower without it is a replica nothing can fall
// back on. What is paced is the copy ahead of the set. See Copier.Ahead.
type Pacer struct {
	limiter *rate.Limiter
}

// NewPacer returns a pacer for bytesPerSecond, or nil — no pacing — for zero or less.
func NewPacer(bytesPerSecond int) *Pacer {
	if bytesPerSecond <= 0 {
		return nil
	}
	return &Pacer{limiter: rate.NewLimiter(rate.Limit(bytesPerSecond), paceBurst)}
}

// PacedRegistryOptions is RegistryOptions with every response body read through pacer. A nil
// pacer gives exactly RegistryOptions.
func PacedRegistryOptions(certificateAuthority, username, password string, pacer *Pacer) ([]remote.Option, error) {
	transport, err := roundTripper(certificateAuthority)
	if err != nil {
		return nil, err
	}

	options := []remote.Option{remote.WithTransport(pacer.wrap(transport))}
	if username != "" || password != "" {
		options = append(options, remote.WithAuth(&authn.Basic{Username: username, Password: password}))
	}
	return options, nil
}

// wrap returns transport with every response body read through the pacer; a nil pacer returns it
// unchanged.
func (p *Pacer) wrap(transport http.RoundTripper) http.RoundTripper {
	if p == nil {
		return transport
	}
	return &pacedTransport{base: transport, limiter: p.limiter}
}

type pacedTransport struct {
	base    http.RoundTripper
	limiter *rate.Limiter
}

func (t *pacedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response.Body == nil {
		return response, err
	}
	response.Body = &pacedBody{ReadCloser: response.Body, limiter: t.limiter, ctx: request.Context()}
	return response, nil
}

type pacedBody struct {
	io.ReadCloser
	limiter *rate.Limiter
	ctx     context.Context
}

func (b *pacedBody) Read(buffer []byte) (int, error) {
	if len(buffer) > paceBurst {
		buffer = buffer[:paceBurst]
	}
	n, err := b.ReadCloser.Read(buffer)
	if n > 0 {
		if waitErr := b.limiter.WaitN(b.ctx, n); waitErr != nil && err == nil {
			err = waitErr
		}
	}
	return n, err
}
