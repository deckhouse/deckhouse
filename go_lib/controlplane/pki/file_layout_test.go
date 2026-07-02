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
