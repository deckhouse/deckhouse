// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fastping

import (
	"fmt"
	"strings"
	"time"
)

func genIdentifier() int {
	return int(time.Now().UnixNano() & 0xffff)
}

func timeToBytes(t time.Time) []byte {
	ts, _ := t.MarshalBinary()
	return ts
}

func bytesToTime(b []byte) time.Time {
	var t time.Time
	_ = t.UnmarshalBinary(b)
	return t
}

func makeKey(host string, seq int) string {
	cleanHost := strings.TrimSpace(host)
	return fmt.Sprintf("%s:%d", cleanHost, seq)
}

func computeChecksum(data []byte) uint16 {
	var sum uint32
	length := len(data)

	for i := 0; i < length-1; i += 2 {
		sum += uint32(data[i])<<8 + uint32(data[i+1])
	}
	if length%2 == 1 {
		sum += uint32(data[length-1]) << 8
	}

	sum = (sum >> 16) + (sum & 0xFFFF)
	sum += sum >> 16
	return uint16(^sum)
}
