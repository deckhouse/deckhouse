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

package commands

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

// validateDeckhouseVersion validates that dhctl version matches Deckhouse version in cluster.
// For destroy operations, use allowAnyError=true to allow destruction even if version check fails.
func validateDeckhouseVersion(ctx context.Context, sshClient node.SSHClient, allowAnyError bool) error {
	return log.Process("default", "Validate version compatibility", func() error {
		kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(sshClient))
		if err != nil {
			if allowAnyError {
				log.WarnF("Could not connect to Kubernetes API for version check: %v\n", err)
				log.WarnLn("Continuing despite connection failure (allowAnyError=true)")
				return nil
			}
			return fmt.Errorf("connect to Kubernetes API for version check: %w", err)
		}
		defer func() {
			if kubeCl.KubeProxy != nil {
				kubeCl.KubeProxy.StopAll()
			}
		}()

		opts := kubernetes.VersionCheckOptions{
			AllowAnyError:       allowAnyError,
			AllowMissingVersion: allowAnyError, // Allow missing version for destroy operations
		}
		return kubernetes.CheckDeckhouseVersionCompatibility(ctx, kubeCl, opts)
	})
}
