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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// handoffRequestTimeout bounds a single collection attempt. The body is a
	// few kilobytes; anything slower is a node that is not ready.
	handoffRequestTimeout = 30 * time.Second

	// handoffResponseLimit caps what is read from the endpoint. A kubeconfig is
	// a few kilobytes and dhctl parses whatever comes back.
	handoffResponseLimit = 1 << 20
)

var (
	// ErrHandoffUnauthorized means the node rejected the token. dhctl and the
	// node hold different payloads, and no amount of waiting fixes that.
	ErrHandoffUnauthorized = errors.New(
		"the handoff endpoint rejected the bootstrap token: the master booted with a payload other than the one in the state cache",
	)

	// ErrHandoffAlreadyServed means the credentials have been collected before.
	// The endpoint serves once and then closes; there is nothing left to wait
	// for.
	ErrHandoffAlreadyServed = errors.New(
		"the handoff endpoint has already handed the cluster credentials over: it serves once and is closed for good",
	)
)

type FetchKubeconfigInput struct {
	// Address is the host:port dhctl reaches the endpoint on, either the master
	// directly or the local end of a tunnel through the bastion.
	Address string
	// ServerName is the name the endpoint's certificate is issued for. It is
	// deliberately not Address: the certificate was issued before the VM
	// existed.
	ServerName string
	// Material carries the CA the endpoint is verified against and the token
	// that authorises the collection.
	Material *HandoffMaterial
}

type Status struct {
	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`
}

// Phase is what the node says it is doing. A named type rather than bare
// strings: the comparisons live in another package, and between two untyped
// strings any typo compiles and quietly never matches.
type Phase string

// The phases the node reports that dhctl acts on. PhaseReady means the
// kubeconfig can be collected; every other value the node sends, "Preparing"
// among them, is "still working" to the wait.
const (
	PhaseReady     Phase = "Ready"
	PhaseFailed    Phase = "Failed"
	PhaseCollected Phase = "Collected"
)

// FetchStatus asks the node what it is doing. The channel opens when the work
// starts, so this answers long before there is a kubeconfig — letting the
// installer tell a node still generating its PKI from one that died early.
func FetchStatus(ctx context.Context, in FetchKubeconfigInput) (*Status, error) {
	body, err := in.call(ctx, http.MethodGet, statusPath)
	if err != nil {
		return nil, err
	}
	status := &Status{}
	if err := json.Unmarshal(body, status); err != nil {
		return nil, fmt.Errorf("parse the bootstrap status from the node: %w", err)
	}
	return status, nil
}

// FetchKubeconfig collects the admin kubeconfig from the node. Collecting does
// not end the handover — ConfirmCollected does — so an installer whose
// connection dies mid-read can simply ask again.
func FetchKubeconfig(ctx context.Context, in FetchKubeconfigInput) ([]byte, error) {
	body, err := in.call(ctx, http.MethodGet, handoffPath)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("the handoff endpoint returned an empty admin kubeconfig")
	}
	return body, nil
}

// ConfirmCollected tells the node the kubeconfig is safely in hand. Only this
// ends the handover: a response that reached the socket says nothing about
// whether the installer kept it, so the node keeps the channel open until then.
func ConfirmCollected(ctx context.Context, in FetchKubeconfigInput) error {
	_, err := in.call(ctx, http.MethodPost, collectedPath)
	return err
}

func (in FetchKubeconfigInput) call(ctx context.Context, method, path string) ([]byte, error) {
	endpointMissing := in.Address == "" || in.ServerName == ""
	if endpointMissing || in.Material == nil {
		return nil, errors.New("reach the node's bootstrap channel: address, server name or handoff material is empty")
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(in.Material.CACertPEM)) {
		return nil, errors.New("the handoff CA certificate PEM holds no certificate")
	}

	request, err := http.NewRequestWithContext(ctx, method, "https://"+in.Address+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build the request to the node's bootstrap channel: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+in.Material.Token)

	client := &http.Client{
		Timeout: handoffRequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				ServerName: in.ServerName,
				MinVersion: tls.VersionTLS12,
			},
		},
	}
	defer client.CloseIdleConnections()

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("reach the node's bootstrap channel at %s: %w", in.Address, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, handoffResponseLimit))
	if err != nil {
		return nil, fmt.Errorf("read the answer from the node's bootstrap channel: %w", err)
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return body, nil
	case http.StatusUnauthorized:
		return nil, ErrHandoffUnauthorized
	case http.StatusGone:
		return nil, ErrHandoffAlreadyServed
	default:
		return nil, fmt.Errorf("the node's bootstrap channel answered %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
}
