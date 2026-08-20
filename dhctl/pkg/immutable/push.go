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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// pushTimeout bounds one PUT, and the node writes the document to its config
// partition before answering — a disk write, not a download.
const pushTimeout = 30 * time.Second

// maxPushErrorBody caps how much of a failing response is quoted back.
const maxPushErrorBody = 512

// errPathUnknown means the server on the port does not serve the path, so
// whatever holds it is neither olcedar-init nor the agent.
var errPathUnknown = errors.New("the maintenance server does not serve this path")

// Whoami tells which of the two servers holds :50000 on a machine: the
// installer waiting for a configuration, or the agent of a node already
// installed. Asked instead of inferred, so nothing has to be pushed to find out.
func Whoami(ctx context.Context, address string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+whoamiPath, nil)
	if err != nil {
		return "", fmt.Errorf("build the whoami request for %s: %w", address, err)
	}

	client := &http.Client{Timeout: whoamiTimeout}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("ask %s who holds the port: %w", address, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s to %s: %w", address, response.Status, whoamiPath, errPathUnknown)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxPushErrorBody))
	if err != nil {
		return "", fmt.Errorf("read who holds %s: %w", address, err)
	}
	return strings.TrimSpace(string(body)), nil
}

// whoamiTimeout bounds the identity question: it reads nothing and writes
// nothing, so an answer that takes long is an answer from something else.
const whoamiTimeout = 5 * time.Second

// The two answers /whoami gives.
const (
	WhoamiInstaller = "installer"
	WhoamiAgent     = "agent"
)

// ErrMaintenanceTokenRequired means the port is held by the agent of a node
// that is already installed. Waiting cannot change that, so a caller must not
// retry it: the machine has a configuration and may not be handed another.
var ErrMaintenanceTokenRequired = errors.New(
	"the maintenance endpoint asks for a token: the port is held by the agent of a node that is already installed",
)

// PushNodeConfig hands a node waiting in maintenance the document it boots
// from. The endpoint is unauthenticated by design — the machine holds no secret
// at this point — so the caller answers for the network the address lives on.
func PushNodeConfig(ctx context.Context, address string, document []byte) error {
	return putNodeConfig(ctx, address, nodeConfigPushPath, document)
}

func putNodeConfig(ctx context.Context, address, path string, document []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://"+address+path, bytes.NewReader(document))
	if err != nil {
		return fmt.Errorf("build the push request for %s: %w", address, err)
	}
	request.Header.Set("Content-Type", "application/yaml")

	client := &http.Client{Timeout: pushTimeout}
	defer client.CloseIdleConnections()

	response, err := client.Do(request)
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
		return fmt.Errorf("%s does not serve %s: %w", address, path, errPathUnknown)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxPushErrorBody))
	if err != nil {
		return fmt.Errorf("push the node configuration to %s: %s: read the refusal: %w", address, response.Status, err)
	}
	return fmt.Errorf("push the node configuration to %s: %s: %s", address, response.Status, bytes.TrimSpace(body))
}
