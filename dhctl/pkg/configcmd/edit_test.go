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

package configcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
)

// dhctl's edit reaches the cluster over SSH too; deckhouse-controller's runs in it, kube flags only.
func TestEditCommandFlags(t *testing.T) {
	sshFlags := []string{"ssh-host", "ssh-user", "connection-config", "ask-become-pass"}

	for name := range editSecrets {
		leaf := &cobra.Command{Use: name}
		DefineEditCommand(leaf, options.New())

		for _, flag := range append([]string{"kubeconfig"}, sshFlags...) {
			require.NotNil(t, leaf.Flags().Lookup(flag), "%s --%s", leaf.Name(), flag)
		}
	}

	controllerEdit := &cobra.Command{Use: "edit"}
	DefineKubeEditCommands(controllerEdit, options.New())
	require.Len(t, controllerEdit.Commands(), 3)

	for _, leaf := range controllerEdit.Commands() {
		require.NotNil(t, leaf.Flags().Lookup("kubeconfig"), leaf.Name())

		for _, name := range sshFlags {
			require.Nil(t, leaf.Flags().Lookup(name), "%s --%s", leaf.Name(), name)
		}
	}
}

// Without the kube flags the kube-only edit has no way to Kubernetes; the error names no SSH.
func TestKubeEditWithoutKubeFlags(t *testing.T) {
	// A context names nothing to connect to on its own.
	for _, args := range [][]string{{"cluster-configuration"}, {"cluster-configuration", "--kubeconfig-context", "foo"}} {
		edit := &cobra.Command{Use: "edit"}
		DefineKubeEditCommands(edit, options.New())
		edit.SetArgs(args)
		edit.SetOut(io.Discard)
		edit.SetErr(io.Discard)

		require.EqualError(t, edit.Execute(), "no Kubernetes connection: give --kubeconfig or --kube-client-from-cluster", args)
	}
}

// An absent Secret is created with the labels bootstrap gives it: the validating webhook selects it by them.
func TestEditCreatesSecretWithBootstrapLabels(t *testing.T) {
	const secret = "d8-static-cluster-configuration"

	opts := options.New()
	opts.Global.CandiDir = tests.RequireCandiDir(t)
	opts.Global.EnsureCandiAvailable = false
	opts.Global.TmpDir = t.TempDir()
	opts.Render.Editor = filepath.Join(t.TempDir(), "editor")
	editor := "#!/bin/sh\nprintf 'apiVersion: deckhouse.io/v1\\nkind: StaticClusterConfiguration\\n' > \"$1\"\n"
	require.NoError(t, os.WriteFile(opts.Render.Editor, []byte(editor), 0o700))

	kubeCl := client.NewFakeKubernetesClient()

	require.NoError(t, editSecret(t.Context(), kubeCl, opts, "static-cluster-configuration", secret, "static-cluster-configuration.yaml"))

	created, err := kubeCl.KubeClient.CoreV1().Secrets("kube-system").Get(t.Context(), secret, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"name": secret}, created.Labels)
}
