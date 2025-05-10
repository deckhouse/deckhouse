/*
Copyright 2025 Flant JSC

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

package fastping

import (
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"time"
)

func genIdentifier() int {
	return int(time.Now().UnixNano() & 0xffff)
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

// extractTimestampNS parses the control message and extracts SO_TIMESTAMPNS (nanosecond precision kernel timestamp).
func extractTimestampNS(oob []byte) time.Time {
	cmsgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return time.Time{}
	}

	for _, cmsg := range cmsgs {
		if cmsg.Header.Level == syscall.SOL_SOCKET && cmsg.Header.Type == syscall.SO_TIMESTAMPNS {
			if len(cmsg.Data) >= 16 {
				// Linux returns timespec as two uint64: sec + nsec
				sec := int64(binary.LittleEndian.Uint64(cmsg.Data[0:8]))
				nsec := int64(binary.LittleEndian.Uint64(cmsg.Data[8:16]))
				return time.Unix(sec, nsec)
			}
		}
	}
	return time.Time{}
}
