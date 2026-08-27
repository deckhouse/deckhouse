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

package immutable

import (
	"os"
	"strings"
	"testing"
)

// A wait rebuilds this channel on every attempt, and the provider prints three
// lines per open. Taking the logger from the context is what lets the caller
// quiet the plumbing behind its own progress line.
func TestTheTunnelNarratesWhereItsContextNarrates(t *testing.T) {
	src, err := os.ReadFile("tunnel.go")
	if err != nil {
		t.Fatalf("read tunnel.go: %v", err)
	}

	body := string(src)
	if !strings.Contains(body, "channelSettings{Settings: sett, logger: dhlog.FromContext(ctx)}") {
		t.Error("the SSH provider must take its logger from the context, or a wait cannot quiet the plumbing behind it")
	}
	if strings.Contains(body, "provider.NewDefaultSSHProvider(\n\t\tsett,") {
		t.Error("the bare settings narrate onto the screen whatever the caller asked for")
	}
}
