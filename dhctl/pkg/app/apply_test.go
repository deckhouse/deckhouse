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

package app

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type testFlags struct {
	LoggerType string
	Config     []string
	Timeout    time.Duration
	Phases     []string
	Labels     map[string]string
}

func newTestApp() (*cobra.Command, *testFlags) {
	f := &testFlags{LoggerType: "pretty", Timeout: 15 * time.Minute}
	root := &cobra.Command{Use: "dhctl", SilenceUsage: true, SilenceErrors: true}
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return ApplyEnvironment(cmd)
	}
	EnumFlag(root.PersistentFlags(), Flag{Name: "logger-type", Env: "T_LOGGER_TYPE"}, &f.LoggerType, "pretty", "json")

	bootstrap := &cobra.Command{Use: "bootstrap", RunE: func(*cobra.Command, []string) error { return nil }}
	StringsFlag(bootstrap.Flags(), Flag{Name: "config", Env: "T_CONFIG"}, &f.Config)
	DurationFlag(bootstrap.Flags(), Flag{Name: "resources-timeout", Env: "T_RESOURCES_TIMEOUT"}, &f.Timeout)
	StringsFlag(bootstrap.Flags(), Flag{Name: "skip-phase", Env: "T_SKIP_PHASES"}, &f.Phases)
	PairsFlag(bootstrap.Flags(), Flag{Name: "kube-cachestore-labels", Env: "T_LABELS"}, &f.Labels)
	root.AddCommand(bootstrap)

	return root, f
}

func runTestApp(args ...string) (*testFlags, error) {
	root, f := newTestApp()
	root.SetArgs(args)

	return f, root.Execute()
}

func TestApplyEnvironment(t *testing.T) {
	t.Run("environment fills what the command line left unset", func(t *testing.T) {
		t.Setenv("T_RESOURCES_TIMEOUT", "1m")

		f, err := runTestApp("bootstrap")
		require.NoError(t, err)
		require.Equal(t, time.Minute, f.Timeout)
	})
	t.Run("command line wins", func(t *testing.T) {
		t.Setenv("T_RESOURCES_TIMEOUT", "1m")

		f, err := runTestApp("bootstrap", "--resources-timeout", "2m")
		require.NoError(t, err)
		require.Equal(t, 2*time.Minute, f.Timeout)
	})
	t.Run("empty environment counts as unset", func(t *testing.T) {
		t.Setenv("T_RESOURCES_TIMEOUT", "")

		f, err := runTestApp("bootstrap")
		require.NoError(t, err)
		require.Equal(t, 15*time.Minute, f.Timeout)
	})
	t.Run("environment reaches a flag of the root", func(t *testing.T) {
		t.Setenv("T_LOGGER_TYPE", "json")

		f, err := runTestApp("bootstrap")
		require.NoError(t, err)
		require.Equal(t, "json", f.LoggerType)
	})
	t.Run("list from the command line replaces the environment", func(t *testing.T) {
		t.Setenv("T_SKIP_PHASES", "a")

		f, err := runTestApp("bootstrap", "--skip-phase", "x")
		require.NoError(t, err)
		require.Equal(t, []string{"x"}, f.Phases)
	})
	t.Run("list and map from the environment split on newlines", func(t *testing.T) {
		t.Setenv("T_SKIP_PHASES", "a\nb\n")
		t.Setenv("T_LABELS", "team=x\nmode=y")

		f, err := runTestApp("bootstrap")
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, f.Phases)
		require.Equal(t, map[string]string{"team": "x", "mode": "y"}, f.Labels)
	})
	t.Run("a lone newline in the environment is one empty item, as in kingpin", func(t *testing.T) {
		t.Setenv("T_CONFIG", "\n")

		f, err := runTestApp("bootstrap")
		require.NoError(t, err)
		require.Equal(t, []string{""}, f.Config)
	})
	t.Run("a second apply changes nothing", func(t *testing.T) {
		t.Setenv("T_SKIP_PHASES", "a\nb")

		root, f := newTestApp()
		cmd, _, err := root.Find([]string{"bootstrap"})
		require.NoError(t, err)
		require.NoError(t, ApplyEnvironment(cmd))
		require.NoError(t, ApplyEnvironment(cmd))
		require.Equal(t, []string{"a", "b"}, f.Phases)
	})
	t.Run("a bad environment value names the variable and its flag", func(t *testing.T) {
		t.Setenv("T_RESOURCES_TIMEOUT", "soon")

		_, err := runTestApp("bootstrap")
		require.ErrorContains(t, err, "T_RESOURCES_TIMEOUT for --resources-timeout: time: invalid duration")
	})
}

// A variable gives a required flag as argv does: cobra checks the required flags after the pre-run.
func TestApplyEnvironmentRequired(t *testing.T) {
	execute := func(args ...string) error {
		var name string

		root := &cobra.Command{Use: "dhctl", SilenceUsage: true, SilenceErrors: true}
		root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
			return ApplyEnvironment(cmd)
		}
		cmd := &cobra.Command{Use: "run", RunE: func(*cobra.Command, []string) error { return nil }}
		StringFlag(cmd.Flags(), Flag{Name: "name", Env: "T_NAME", Required: true}, &name)
		root.AddCommand(cmd)
		root.SetArgs(append([]string{"run"}, args...))

		return root.Execute()
	}

	t.Run("absent everywhere", func(t *testing.T) {
		require.EqualError(t, execute(), `required flag(s) "name" not set`)
	})
	t.Run("on the command line", func(t *testing.T) {
		require.NoError(t, execute("--name", "x"))
	})
	t.Run("in the environment", func(t *testing.T) {
		t.Setenv("T_NAME", "x")
		require.NoError(t, execute())
	})
}
