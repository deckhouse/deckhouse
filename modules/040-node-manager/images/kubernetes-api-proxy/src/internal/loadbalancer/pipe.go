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

package loadbalancer

import (
	"io"
	"net"

	"github.com/siderolabs/tcpproxy"
)

func copyToUpstream(errc chan<- error, r *relay, src net.Conn) {
	_, err := io.Copy(r, src)
	if err != nil {
		_ = r.conn().Close()
	}

	r.closeWrite()
	closeRead(src)

	errc <- err
}

func copyToClient(errc chan<- error, dst, src net.Conn) {
	_, err := io.Copy(dst, src)

	closeWrite(dst)
	closeRead(src)

	errc <- err
}

func waitBoth(errc <-chan error, src net.Conn, r *relay) {
	for range 2 {
		if err := <-errc; err != nil {
			closeBoth(src, r)
		}
	}

	closeBoth(src, r)
}

func closeBoth(src net.Conn, r *relay) {
	_ = src.Close()
	_ = r.conn().Close()
}

func closeWrite(c net.Conn) {
	if tcp, ok := tcpproxy.UnderlyingConn(c).(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}

func closeRead(c net.Conn) {
	if tcp, ok := tcpproxy.UnderlyingConn(c).(*net.TCPConn); ok {
		_ = tcp.CloseRead()
	}
}
