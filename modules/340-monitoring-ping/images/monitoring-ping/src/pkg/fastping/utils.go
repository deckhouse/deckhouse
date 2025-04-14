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
