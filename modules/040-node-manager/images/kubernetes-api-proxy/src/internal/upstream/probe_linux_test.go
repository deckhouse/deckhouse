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

package upstream

import (
	"context"
	"net"
	"net/http/httptrace"
	"os"
	"testing"
	"time"

	"github.com/siderolabs/tcpproxy"
	"k8s.io/client-go/rest"
)

// The probe connection has TCP_USER_TIMEOUT equal to the health timeout.
func TestList_ProbeSocketUserTimeout(t *testing.T) {
	f := newProbeFixture(t)

	const timeout = 300 * time.Millisecond

	list, err := newList(nil, byAddr, WithHealthcheckTimeout(timeout))
	if err != nil {
		t.Fatal(err)
	}

	// With certificate files, client-go wraps the connection. The data from
	// the files gives the TCP connection to the trace.
	tlsConfig := rest.TLSClientConfig{
		CAData:   mustRead(t, f.files.ca),
		CertData: mustRead(t, f.files.cert),
		KeyData:  mustRead(t, f.files.key),
	}
	u := NewUpstream(f.server.addr(), func() (*rest.Config, error) {
		return &rest.Config{TLSClientConfig: tlsConfig}, nil
	})

	var conn net.Conn

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { conn = info.Conn },
	})

	if r, err := list.probe(ctx, u); err != nil || r.kind != probeReady {
		t.Fatalf("probe() = %v, %v, want %v", r.kind, err, probeReady)
	}

	tlsConn, ok := conn.(interface{ NetConn() net.Conn })
	if !ok {
		t.Fatalf("connection %T is not TLS", conn)
	}

	got, err := tcpproxy.GetTCPUserTimeout(tlsConn.NetConn())
	if err != nil {
		t.Fatal(err)
	}

	if want := int(timeout / time.Millisecond); got != want {
		t.Errorf("TCP_USER_TIMEOUT = %d ms, want %d ms", got, want)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}
