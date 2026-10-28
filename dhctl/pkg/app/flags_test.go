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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func newFlagTestCommand() *cobra.Command {
	return &cobra.Command{Use: "x", SilenceUsage: true, SilenceErrors: true, RunE: func(*cobra.Command, []string) error { return nil }}
}

func TestEnumFlag(t *testing.T) {
	cmd := newFlagTestCommand()
	v := "pretty"
	EnumFlag(cmd.Flags(), Flag{Name: "logger-type"}, &v, "pretty", "json")
	require.Equal(t, "pretty", cmd.Flags().Lookup("logger-type").DefValue)

	cmd.SetArgs([]string{"--logger-type", "xml"})
	require.ErrorContains(t, cmd.Execute(), "enum value must be one of pretty,json, got 'xml'")
	cmd.SetArgs([]string{"--logger-type", "json"})
	require.NoError(t, cmd.Execute())
	require.Equal(t, "json", v)
}

func TestPairsFlag(t *testing.T) {
	cmd := newFlagTestCommand()

	var m map[string]string
	PairsFlag(cmd.Flags(), Flag{Name: "labels"}, &m)
	require.Equal(t, "", cmd.Flags().Lookup("labels").DefValue)

	cmd.SetArgs([]string{"--labels", "a=b=c", "--labels", "d=e"})
	require.NoError(t, cmd.Execute())
	require.Equal(t, map[string]string{"a": "b=c", "d": "e"}, m)
	cmd.SetArgs([]string{"--labels", "novalue"})
	require.ErrorContains(t, cmd.Execute(), "expected KEY=VALUE")
}

// kingpin took --no-<name> for a bool flag: it turns the flag off and counts as giving it, so the
// environment does not override it. A flag that is not a bool has no --no-<name>.
func TestBoolFlagNegation(t *testing.T) {
	t.Setenv("TEST_FAIL_FAST", "true")

	cmd := newFlagTestCommand()
	failFast := true
	BoolFlag(cmd.Flags(), Flag{Name: "preflight-fail-fast", Env: "TEST_FAIL_FAST"}, &failFast)

	cmd.SetArgs([]string{"--no-preflight-fail-fast"})
	require.NoError(t, cmd.Execute())
	require.NoError(t, ApplyEnvironment(cmd))
	require.False(t, failFast)
	require.True(t, cmd.Flags().Changed("preflight-fail-fast"))
	require.NotContains(t, cmd.Flags().FlagUsages(), "no-preflight-fail-fast")

	cmd = newFlagTestCommand()

	var user string
	StringFlag(cmd.Flags(), Flag{Name: "ssh-user"}, &user)
	cmd.SetArgs([]string{"--no-ssh-user", "x"})
	require.EqualError(t, cmd.Execute(), "unknown flag: --no-ssh-user")
}

// kingpin refused a flag of one value given twice, its --no-<name> included; a list or a map
// takes every occurrence.
func TestScalarFlagRepeated(t *testing.T) {
	run := func(args ...string) error {
		cmd := newFlagTestCommand()

		var (
			user, logger string
			verbose      bool
			port         int
			timeout      time.Duration
			hosts        []string
			labels       map[string]string
		)
		StringFlag(cmd.Flags(), Flag{Name: "ssh-user"}, &user)
		EnumFlag(cmd.Flags(), Flag{Name: "logger-type"}, &logger, "pretty", "json")
		BoolFlag(cmd.Flags(), Flag{Name: "verbose", Short: "v"}, &verbose)
		IntFlag(cmd.Flags(), Flag{Name: "port"}, &port)
		DurationFlag(cmd.Flags(), Flag{Name: "timeout"}, &timeout)
		StringsFlag(cmd.Flags(), Flag{Name: "ssh-host"}, &hosts)
		PairsFlag(cmd.Flags(), Flag{Name: "labels"}, &labels)
		cmd.SetArgs(args)

		return cmd.Execute()
	}

	require.ErrorContains(t, run("--ssh-user", "a", "--ssh-user", "b"), "flag 'ssh-user' cannot be repeated")
	require.ErrorContains(t, run("--logger-type", "json", "--logger-type", "json"), "flag 'logger-type' cannot be repeated")
	require.ErrorContains(t, run("--port", "1", "--port", "2"), "flag 'port' cannot be repeated")
	require.ErrorContains(t, run("--timeout", "1s", "--timeout", "2s"), "flag 'timeout' cannot be repeated")
	require.ErrorContains(t, run("-v", "-v"), "flag 'verbose' cannot be repeated")
	require.ErrorContains(t, run("--verbose", "--no-verbose"), "flag 'verbose' cannot be repeated")
	require.ErrorContains(t, run("--no-verbose", "--verbose"), "flag 'verbose' cannot be repeated")
	require.ErrorContains(t, run("--no-verbose", "--no-verbose"), "flag 'verbose' cannot be repeated")
	require.NoError(t, run("--ssh-host", "a", "--ssh-host", "b", "--labels", "a=b", "--labels", "c=d"))
}

// The environment sets a flag argv left unchanged, once.
func TestScalarFlagFromEnvironment(t *testing.T) {
	t.Setenv("TEST_USER", "env")
	t.Setenv("TEST_PORT", "2")

	cmd := newFlagTestCommand()

	var user string

	port := 1

	StringFlag(cmd.Flags(), Flag{Name: "ssh-user", Env: "TEST_USER"}, &user)
	IntFlag(cmd.Flags(), Flag{Name: "port", Env: "TEST_PORT"}, &port)
	cmd.SetArgs([]string{"--ssh-user", "argv"})

	require.NoError(t, cmd.Execute())
	require.NoError(t, ApplyEnvironment(cmd))
	require.Equal(t, "argv", user)
	require.Equal(t, 2, port)
}

func TestSplitEnvList(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, splitEnvList("a\nb\n"))
	require.Equal(t, []string{"a", "b"}, splitEnvList("a\r\nb\r\n"))
	require.Equal(t, []string{"a,b"}, splitEnvList("a,b"))
	require.Equal(t, []string{""}, splitEnvList("\n"))
}

// pflag takes the first back-quoted word of the help as the placeholder, as kingpin PlaceHolder did.
func TestHelpPlaceholders(t *testing.T) {
	cmd := newFlagTestCommand()
	DefinePreflight(cmd, &options.PreflightOptions{})
	DefineSkipPhases(cmd, &options.BootstrapOptions{})

	usages := cmd.Flags().FlagUsages()
	require.Contains(t, usages, "--preflight-skip-check name ")
	require.Contains(t, usages, "--skip-phase phase ")
}
