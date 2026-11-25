// Copyright 2026 Flant JSC
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

package immutable

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// pushTimeout bounds one PUT, and the node writes the document to its config
// partition before answering — a disk write, not a download.
const pushTimeout = 30 * time.Second

// maxPushErrorBody caps how much of a failing response is quoted back.
const maxPushErrorBody = 512

// errPathUnknown means the server on the port does not serve the path, so
// whatever holds it is neither the Deckhouse Engine init nor the agent.
var errPathUnknown = errors.New("the maintenance server does not serve this path")

// ErrMaintenanceTokenRequired means the port is held by the agent of a node
// that is already installed. Waiting cannot change that, so a caller must not
// retry it: the machine has a configuration and may not be handed another.
var ErrMaintenanceTokenRequired = errors.New(
	"the maintenance endpoint asks for a token: the port is held by the agent of a node that is already installed",
)

// do runs one request against a machine's maintenance server and hands the
// answer back for the caller to classify: each endpoint reads its statuses
// differently. The caller closes the body.
func do(ctx context.Context, method, url, token string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build the request: %w", err)
	}
	// Only the document is ever sent as a body, and the node reads it as YAML.
	if body != nil {
		request.Header.Set("Content-Type", "application/yaml")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	// A transport of its own, never the process-wide default: that one proxies
	// through HTTP_PROXY, and a machine waiting for its configuration is on the
	// provisioning network, not behind a proxy that would be handed the payload.
	// The agent's certificate is self-signed and nothing here can pin it; the
	// bearer tokens authenticate, TLS only keeps them off the wire.
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // self-signed agent certificate, see above
		MinVersion:         tls.VersionTLS12,
	}}
	client := &http.Client{Timeout: pushTimeout, Transport: transport}
	defer client.CloseIdleConnections()

	return client.Do(request)
}

// doMaintenance asks the server on a machine's maintenance port. The agent of an
// installed node serves it over HTTPS (nodelet internal/services/maintenance);
// the Deckhouse Engine init and an older agent over plain HTTP, so a plain answer
// to the TLS handshake sends the same request again over HTTP.
func doMaintenance(ctx context.Context, method, address, path, token string, body []byte) (*http.Response, error) {
	response, err := do(ctx, method, "https://"+address+path, token, body)
	if errors.Is(err, http.ErrSchemeMismatch) {
		return do(ctx, method, "http://"+address+path, token, body)
	}
	return response, err
}

// errorQuote is as much of a refusal as is worth repeating back to the
// operator. A body that cannot be read is no second failure: the status the
// caller prints beside it already says the request was refused.
func errorQuote(response *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPushErrorBody))
	if err != nil {
		return "the refusal could not be read: " + err.Error()
	}
	return string(bytes.TrimSpace(body))
}

// PushNodeConfig hands a node waiting in maintenance the document it boots
// from. The endpoint is unauthenticated by design — the machine holds no secret
// at this point — so the caller answers for the network the address lives on.
func PushNodeConfig(ctx context.Context, address string, document []byte) error {
	response, err := doMaintenance(ctx, http.MethodPut, address, nodeConfigPushPath, "", document)
	if err != nil {
		return fmt.Errorf("push the node configuration to %s: %w", address, err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	// The agent of an installed node holds the same port and the same path, but
	// demands its maintenance token.
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("push the node configuration to %s: %w", address, ErrMaintenanceTokenRequired)
	}
	if response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s does not serve %s: %w", address, nodeConfigPushPath, errPathUnknown)
	}

	return fmt.Errorf("push the node configuration to %s: %s: %s", address, response.Status, errorQuote(response))
}

// AgentHoldsPort reports that the port is held by the agent of a node that
// already has its configuration, rather than by the init waiting for one.
//
// Asked without a token, and answered without one: the endpoint says which of
// the two servers is running, never which machine this is. A machine that does
// not answer it at all is not an agent as far as this is concerned — an older
// image serves no such path, and the caller checks it the long way.
func AgentHoldsPort(ctx context.Context, address string) (bool, error) {
	response, err := doMaintenance(ctx, http.MethodGet, address, whoamiPath, "", nil)
	if err != nil {
		return false, fmt.Errorf("ask %s which server holds its maintenance port: %w", address, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false, nil
	}

	role, err := io.ReadAll(io.LimitReader(response.Body, maxPushErrorBody))
	if err != nil {
		return false, fmt.Errorf("read what holds the maintenance port of %s: %w", address, err)
	}
	return string(bytes.TrimSpace(role)) == agentRole, nil
}

// agentRole is what the agent's maintenance server answers on whoamiPath.
const agentRole = "agent"
