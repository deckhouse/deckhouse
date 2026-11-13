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

// Package accesslog receives the access log of the pages nginx serves from
// the cache volume.
package accesslog

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Recorder records an access to a version, which delays its eviction
// (ondemand.Store.Touch).
type Recorder interface {
	Touch(module, version string)
}

// Listen receives the access log nginx writes over syslog for the pages it
// serves from the cache volume, and records each access with recorder so
// that opened versions are evicted last. A line ends with the page path,
// /<lang>/modules/<module>/<version>/... Lost datagrams only make a version
// look less recently opened.
func Listen(ctx context.Context, addr string, recorder Recorder, logger *log.Logger) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			logger.Warn("read access log", log.Err(err))
			continue
		}

		if module, version, ok := parse(string(buf[:n])); ok {
			recorder.Touch(module, version)
		}
	}
}

func parse(line string) (string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", "", false
	}

	// "", <lang>, "modules", <module>, <version>, ...
	parts := strings.SplitN(fields[len(fields)-1], "/", 6)
	if len(parts) < 6 || parts[0] != "" || parts[2] != "modules" {
		return "", "", false
	}

	return parts[3], parts[4], true
}
