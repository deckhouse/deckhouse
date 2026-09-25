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

package checks

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestChecksDoNotReadSilenceAsAnAnswer is the lesson of a static bootstrap that reported fourteen
// separate findings — sudo not installed, systemd not running, no package manager, no hostname,
// kernel modules that cannot be loaded — about a node that had all of them. One connection had
// been torn down mid-phase, and every check that asks its question by running a command read the
// command not running as the answer being no.
//
// A check may not conclude anything from a node it could not reach.
func TestChecksDoNotReadSilenceAsAnAnswer(t *testing.T) {
	// What a resolver hands back once the shared connection is gone.
	gone := func() NodeInterfaceFunc {
		return func(ctx context.Context) (libcon.Interface, error) {
			return nil, fmt.Errorf("%w: something closed it", ErrNodeConnectionGone)
		}
	}

	// One entry per check that draws a conclusion from what a command printed. Each is given a
	// node it cannot reach, and none of them may say anything about its subject.
	checks := map[string]func(NodeInterfaceFunc) (string, error){
		"node-os-supported": func(ni NodeInterfaceFunc) (string, error) {
			return NodeOSSupportedCheck{NodeInterface: ni}.Run(t.Context())
		},
		"node-kernel-modules": func(ni NodeInterfaceFunc) (string, error) {
			return NodeKernelModulesCheck{NodeInterface: ni}.Run(t.Context())
		},
		"node-selinux-tools": func(ni NodeInterfaceFunc) (string, error) {
			return NodeSELinuxToolsCheck{NodeInterface: ni}.Run(t.Context())
		},
		"node-resolve-hostname": func(ni NodeInterfaceFunc) (string, error) {
			return NodeResolveHostnameCheck{NodeInterface: ni}.Run(t.Context())
		},
		"node-xfs-ftype": func(ni NodeInterfaceFunc) (string, error) {
			return NodeXFSFtypeCheck{NodeInterface: ni}.Run(t.Context())
		},
		"node-system-requirements": func(ni NodeInterfaceFunc) (string, error) {
			return NodeSystemRequirementsCheck{NodeInterface: ni}.Run(t.Context())
		},
	}

	for name, run := range checks {
		t.Run(name, func(t *testing.T) {
			_, err := run(gone())

			require.Error(t, err)
			// The one cause, reachable however the check wrapped it.
			assert.ErrorIs(t, err, ErrNodeConnectionGone)

			// And emphatically not a verdict about the node. It comes back as a plain error,
			// which the report renders under `reason:` — there is nothing to put in the five
			// fields of a Failure, because nothing was examined.
			var failure *preflight.Failure
			assert.NotErrorAs(t, err, &failure)
			for _, verdict := range []string{"is not installed", "is not running", "printed nothing", "cannot be loaded"} {
				assert.NotContains(t, err.Error(), verdict,
					"a node that could not be reached was not examined")
			}
		})
	}
}
