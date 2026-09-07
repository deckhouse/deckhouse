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

package pki

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileLayout(t *testing.T) {
	want := map[string]string{
		"ca.crt":                       "ca.crt",
		"ca.key":                       "ca.key",
		"front-proxy-ca.crt":           "front-proxy-ca.crt",
		"front-proxy-ca.key":           "front-proxy-ca.key",
		"front-proxy-client.crt":       "front-proxy-client.crt",
		"front-proxy-client.key":       "front-proxy-client.key",
		"apiserver.crt":                "apiserver.crt",
		"apiserver.key":                "apiserver.key",
		"apiserver-kubelet-client.crt": "apiserver-kubelet-client.crt",
		"apiserver-kubelet-client.key": "apiserver-kubelet-client.key",
		"apiserver-etcd-client.crt":    "apiserver-etcd-client.crt",
		"apiserver-etcd-client.key":    "apiserver-etcd-client.key",
		"etcd-ca.crt":                  "etcd/ca.crt",
		"etcd-ca.key":                  "etcd/ca.key",
		"etcd-server.crt":              "etcd/server.crt",
		"etcd-server.key":              "etcd/server.key",
		"etcd-peer.crt":                "etcd/peer.crt",
		"etcd-peer.key":                "etcd/peer.key",
		"etcd-healthcheck-client.crt":  "etcd/healthcheck-client.crt",
		"etcd-healthcheck-client.key":  "etcd/healthcheck-client.key",
		"sa.key":                       "sa.key",
		"sa.pub":                       "sa.pub",
	}
	require.Equal(t, want, FileLayout())
}
