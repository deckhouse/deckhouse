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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// The lock is released in the name of the user the connection config connects as, not $USER.
func TestReleaseLockUserFromConnectionConfig(t *testing.T) {
	t.Setenv("USER", "local")
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "connection-config.yaml")
	body := "apiVersion: dhctl.deckhouse.io/v1\nkind: SSHConfig\nsshUser: ubuntu\nsudoPassword: dh-test-secret-0123456789abcdef\n---\napiVersion: dhctl.deckhouse.io/v1\nkind: SSHHost\nhost: 10.0.0.1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	opts := options.New()
	opts.Global.CandiDir = "../../../../candi/openapi/dhctl"
	opts.Global.TmpDir = t.TempDir()
	cmd := &cobra.Command{Use: "release", SilenceUsage: true, SilenceErrors: true}
	DefineReleaseConvergeLockCommand(cmd, opts)
	cmd.RunE = func(*cobra.Command, []string) error { return nil }
	cmd.SetArgs([]string{"--connection-config", path})

	require.NoError(t, cmd.Execute())
	require.Equal(t, path, opts.SSH.ConnectionConfigPath)
	require.Equal(t, "ubuntu", opts.SSH.User)
}
