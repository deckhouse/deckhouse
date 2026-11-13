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

package proxyclient

import (
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"

	"golang.org/x/sys/unix"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// The probe connection has TCP_USER_TIMEOUT equal to ProbeTimeout.
func TestProbeSocketUserTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	c := New(&rest.Config{TLSClientConfig: rest.TLSClientConfig{CAData: ca}}, fake.NewClientset())

	var conn net.Conn

	ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { conn = info.Conn },
	})

	if !c.Ready(ctx, srv.Listener.Addr().String()) {
		t.Fatal("200 must be ready")
	}

	tlsConn, ok := conn.(interface{ NetConn() net.Conn })
	if !ok {
		t.Fatalf("connection %T is not TLS", conn)
	}

	tcp, ok := tlsConn.NetConn().(*net.TCPConn)
	if !ok {
		t.Fatalf("connection %T is not TCP", tlsConn.NetConn())
	}

	raw, err := tcp.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	var (
		got  int
		gerr error
	)

	if err := raw.Control(func(fd uintptr) {
		got, gerr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT)
	}); err != nil {
		t.Fatal(err)
	}

	if gerr != nil {
		t.Fatal(gerr)
	}

	if want := int(ProbeTimeout.Milliseconds()); got != want {
		t.Errorf("TCP_USER_TIMEOUT = %d ms, want %d ms", got, want)
	}
}
