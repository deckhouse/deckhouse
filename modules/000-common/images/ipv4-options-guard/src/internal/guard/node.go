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

package guard

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// DisableLabel set to "true" on a node switches the guard there to Disabled, reverting all its changes.
const DisableLabel = "ipv4-options-guard.deckhouse.io/disabled"

const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

// NodeLabels reads the labels of a node with the pod service account; a tiny client to avoid client-go.
type NodeLabels struct {
	node   string
	url    string
	client *http.Client
}

func NewNodeLabels(node string) (*NodeLabels, error) {
	ca, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	return &NodeLabels{
		node: node,
		url:  "https://" + net.JoinHostPort(host, port) + "/api/v1/nodes/" + node,
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
		},
	}, nil
}

func (n *NodeLabels) Disabled() (bool, error) {
	// The token is re-read on every call because it is rotated.
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodGet, n.url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	// Metadata only: a full Node is large, and it is fetched from every node on every resync.
	req.Header.Set("Accept", "application/json;as=PartialObjectMetadata;g=meta.k8s.io;v=v1,application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("get node %s: %s", n.node, resp.Status)
	}
	var obj struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return false, err
	}
	return obj.Metadata.Labels[DisableLabel] == "true", nil
}
