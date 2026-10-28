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

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

var errPrepared = errors.New("prepare ran")

// Ambient DHCTL_CLI_* variables would satisfy required flags or override the values under test.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "DHCTL_CLI_") {
			if err := os.Unsetenv(name); err != nil {
				panic(err)
			}
		}
	}

	os.Exit(m.Run())
}

func executeRoot(t *testing.T, opts *options.Options, allowed []string, prepare func(cmd *cobra.Command) error, args ...string) error {
	t.Helper()

	root := buildRoot(commandList, opts, allowed, prepare)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)

	return root.Execute()
}

// stopAtPrepare stops the run where prepare would set it up: the flags and their hooks are done.
func stopAtPrepare(*cobra.Command) error { return errPrepared }

// prepare takes the tmp dir lock and opens the log file: cobra's help and a bare group must not.
// A bare group fails with kingpin's text, as dhctl did before cobra.
func TestPrepareRunsForCommandsOnly(t *testing.T) {
	failing := func(*cobra.Command) error { return errPrepared }
	run := func(args ...string) error { return executeRoot(t, options.New(), nil, failing, args...) }

	require.NoError(t, run("help", "bootstrap"))
	require.NoError(t, run("bootstrap", "--help-long"))
	require.NoError(t, run("terraform", "--help"))
	require.NoError(t, run("--version"))
	// kingpin printed the usage of a bare dhctl and exited 0.
	require.NoError(t, run([]string{}...))
	require.EqualError(t, run("terraform"), "must select a subcommand of 'terraform'")
	require.EqualError(t, run("config", "edit"), "must select a subcommand of 'config edit'")
	require.ErrorIs(t, run("bootstrap", "--config", "x"), errPrepared)
	require.ErrorIs(t, run("terraform", "check"), errPrepared)
	require.EqualError(t, run("terraform", "bogus"), `unknown command "bogus" for "dhctl terraform"`)
}

// Every command runs prepare before its action: a leaf a DefineFunc added itself would not.
func TestEveryCommandRunsPrepare(t *testing.T) {
	for _, path := range commandPaths(buildRoot(commandList, options.New(), nil, stopAtPrepare)) {
		args := strings.Fields(path)
		root := buildRoot(commandList, options.New(), nil, stopAtPrepare)
		leaf, _, err := root.Find(args)
		require.NoError(t, err)

		if leaf.HasSubCommands() {
			continue
		}

		leaf.RunE = func(*cobra.Command, []string) error { return errors.New("ran without prepare") }

		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		err = root.Execute()
		// A required flag left unset stops the run before prepare, and before the action too.
		require.Error(t, err, args)
		require.NotEqual(t, "ran without prepare", err.Error(), args)
	}
}

// A missing required flag stops the run before prepare takes the tmp dir lock and opens the log
// file, as kingpin refused it before any action; its variable counts as given.
func TestRequiredFlagBeforePrepare(t *testing.T) {
	prepared := false
	prepare := func(*cobra.Command) error {
		prepared = true
		return errPrepared
	}

	require.EqualError(t, executeRoot(t, options.New(), nil, prepare, "bootstrap"), `required flag(s) "config" not set`)
	require.False(t, prepared)

	t.Setenv("DHCTL_CLI_CONFIG", "x")
	require.ErrorIs(t, executeRoot(t, options.New(), nil, prepare, "bootstrap"), errPrepared)
}

// A flag its hook refuses stops the run before prepare too: kingpin checked these in PreActions,
// ahead of the tmp dir setup, whose cleaner would wipe the state an earlier run left there.
func TestFlagHookErrorBeforePrepare(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	prepared := false

	prepare := func(*cobra.Command) error {
		prepared = true
		return errPrepared
	}
	for _, args := range [][]string{
		{"bootstrap", "--config", "x", "--skip-phase", "bogus"},
		{"bootstrap", "--config", "x", "--connection-config", "c.yaml", "--ssh-host", "h"},
	} {
		err := executeRoot(t, options.New(), nil, prepare, args...)
		require.Error(t, err, args)
		require.NotErrorIs(t, err, errPrepared, args)
		require.False(t, prepared, args)
	}
}

// The flag hooks run before prepare checks the tmp dir and arms its cleaner: the --connection-config
// hook writes nothing there, so a refusal in prepare leaves no copy of the keys behind.
func TestConnectionConfigKeysAfterPrepare(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "../../../candi/openapi/dhctl")
	tmpDir := filepath.Join(t.TempDir(), "absent")
	_, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	require.NoError(t, err)

	key := strings.ReplaceAll(strings.TrimSpace(string(pem.EncodeToMemory(block))), "\n", "\n    ")
	path := filepath.Join(t.TempDir(), "connection.yaml")
	body := "apiVersion: dhctl.deckhouse.io/v1\nkind: SSHConfig\nsshUser: ubuntu\nsshAgentPrivateKeys:\n- key: |\n    " + key + "\n" +
		"---\napiVersion: dhctl.deckhouse.io/v1\nkind: SSHHost\nhost: h1\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	opts := options.New()
	err = executeRoot(t, opts, nil, stopAtPrepare, "bootstrap", "--config", "x", "--tmp-dir", tmpDir, "--connection-config", path)
	require.ErrorIs(t, err, errPrepared)
	require.NoDirExists(t, tmpDir)
	require.Len(t, opts.SSH.PrivateKeys, 1)
	require.False(t, opts.SSH.PrivateKeys[0].IsPath, "the key stays text until lib-connection connects")
}

// kingpin refused a bare group with the error alone, no usage.
func TestBareGroupPrintsOnlyTheError(t *testing.T) {
	root := buildRoot(commandList, options.New(), nil, func(*cobra.Command) error { return errPrepared })

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"terraform"})

	require.EqualError(t, root.Execute(), "must select a subcommand of 'terraform'")
	require.Empty(t, out.String())
}

// prepare runs once per run and after the flag hooks, which see the environment.
func TestPrepareRunsOnceAfterFlagHooks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USER", "root")
	t.Setenv("DHCTL_CLI_SSH_HOSTS", "a")
	t.Setenv("DHCTL_CLI_SSH_USER", "u")

	opts := options.New()
	calls := 0
	root := buildRoot(commandList, opts, nil, func(*cobra.Command) error {
		calls++

		require.Len(t, opts.SSH.Hosts, 1, "the SSH flag hook ran before prepare")

		return nil
	})
	leaf, _, err := root.Find([]string{"bootstrap-phase", "create-resources"})
	require.NoError(t, err)

	leaf.RunE = func(*cobra.Command, []string) error { return nil }

	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"bootstrap-phase", "create-resources"})

	require.NoError(t, root.Execute())
	require.Equal(t, []string{"a"}, opts.SSH.HostsRaw)
	require.Equal(t, 1, calls)
	require.Len(t, opts.SSH.Hosts, 1)
	require.Equal(t, "u", opts.SSH.BastionUser)
}

func TestPhaseCommandForcesVerbose(t *testing.T) {
	run := func(args ...string) *options.Options {
		opts := options.New()
		require.ErrorIs(t, executeRoot(t, opts, nil, stopAtPrepare, args...), errPrepared)

		return opts
	}

	config := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(config, []byte("kind: ClusterConfiguration\n"), 0o600))

	for _, phase := range []string{"create-resources", "install-deckhouse", "base-infra"} {
		require.True(t, run("bootstrap-phase", phase, "--config", config).Global.ShowProgress, phase)
	}

	require.False(t, run("phase-catalog").Global.ShowProgress)
}

// kingpin's --version worked on every command, --help and --no-version included: the version on
// stderr, before required flags and prepare.
func TestVersionOnEveryCommand(t *testing.T) {
	for _, args := range [][]string{
		{"--version"},
		{"bootstrap", "--version"},
		{"bootstrap", "--no-version"},
		{"bootstrap", "--help", "--version"},
		{"bootstrap-phase", "--version"},
		{"terraform", "check", "--version"},
	} {
		opts := options.New()
		opts.BuildInfo.AppVersion = "v1.2.3"
		root := buildRoot(commandList, opts, nil, func(*cobra.Command) error { return errPrepared })

		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(args)

		require.NoError(t, root.Execute(), args)
		require.Equal(t, "v1.2.3\n", stderr.String(), args)
		require.Empty(t, stdout.String(), args)
	}
}

// Help lists --version with every command, as kingpin did, and none of the --no-<name> flags.
func TestHelpListsVersionNotNegations(t *testing.T) {
	root := buildRoot(commandList, options.New(), nil, func(*cobra.Command) error { return errPrepared })

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"bootstrap", "--help"})

	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "--version")
	require.Contains(t, out.String(), "Show application version.")
	require.Contains(t, out.String(), "--preflight-fail-fast")
	require.NotContains(t, out.String(), "--no-")
}

// Help names the variable of a flag after its help text, as kingpin did: the name is not always
// derived from the flag's. The help text other code reads stays as declared.
func TestHelpNamesFlagVariables(t *testing.T) {
	root := buildRoot(commandList, options.New(), nil, func(*cobra.Command) error { return errPrepared })

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"bootstrap", "--help"})

	require.NoError(t, root.Execute())

	line := func(flag string) string {
		for l := range strings.SplitSeq(out.String(), "\n") {
			if strings.Contains(l, flag+" ") {
				return l
			}
		}

		t.Fatalf("no %s in help:\n%s", flag, out.String())

		return ""
	}
	require.Contains(t, line("--ssh-host"), "SSH destination hosts, can be specified multiple times ($DHCTL_CLI_SSH_HOSTS)")
	require.Contains(t, line("--resources-timeout"), "Timeout to create resources. Experimental. This feature may be deleted in the future. ($DHCTL_CLI_RESOURCES_TIMEOUT) (default 15m0s)")
	require.Contains(t, line("--logger-type"), "($DHCTL_CLI_LOGGER_TYPE)")
	require.NotContains(t, line("--verbose"), "($")

	bootstrap, _, err := root.Find([]string{"bootstrap"})
	require.NoError(t, err)
	require.Equal(t, "SSH destination hosts, can be specified multiple times", bootstrap.Flags().Lookup("ssh-host").Usage)
}

// A standalone dhctl has no image bashrc: its help names the command that prints the script.
func TestHelpListsCompletion(t *testing.T) {
	root := buildRoot(commandList, options.New(), nil, func(*cobra.Command) error { return errPrepared })

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})

	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "Generate the autocompletion script")
}

// kingpin took --no-<name> of every bool flag, the global ones too, and acted on --no-help as on
// --help.
func TestNegatedFlags(t *testing.T) {
	opts := options.New()
	opts.Global.ShowProgress = true
	opts.Preflight.FailFast = true
	err := executeRoot(t, opts, nil, stopAtPrepare, "converge", "--no-verbose", "--no-preflight-fail-fast")
	require.ErrorIs(t, err, errPrepared)
	require.False(t, opts.Global.ShowProgress)
	require.False(t, opts.Preflight.FailFast)

	require.NoError(t, executeRoot(t, options.New(), nil, stopAtPrepare, "bootstrap", "--no-help"))
	require.EqualError(t, executeRoot(t, options.New(), nil, stopAtPrepare, "bootstrap", "--no-ssh-user", "u"),
		"unknown flag: --no-ssh-user")
}

// A flag of one value given twice fails with kingpin's text, before prepare.
func TestRepeatedFlag(t *testing.T) {
	failing := func(*cobra.Command) error { return errPrepared }
	run := func(args ...string) error { return executeRoot(t, options.New(), nil, failing, args...) }

	require.EqualError(t, run("bootstrap", "--ssh-user", "a", "--ssh-user", "b"), "flag 'ssh-user' cannot be repeated")
	require.EqualError(t, run("bootstrap", "-v", "--verbose"), "flag 'verbose' cannot be repeated")
	require.EqualError(t, run("bootstrap", "--verbose", "--no-verbose"), "flag 'verbose' cannot be repeated")
	require.ErrorIs(t, run("bootstrap", "--config", "a", "--config", "b", "--ssh-host", "a", "--ssh-host", "b"), errPrepared)
}

// kingpin took an `@file` argument as the lines of the file, wherever it stood but after `--`.
func TestExpandArgFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

		return path
	}
	inner := write("inner", "json\n")
	outer := write("outer", "bootstrap\r\n# comment\n\n   \n  kept as is\n--logger-type\n@"+inner)

	got, err := expandArgFiles([]string{"@" + outer, "--ssh-user", "@" + inner, "--x=@" + inner, "--", "@" + inner})
	require.NoError(t, err)
	require.Equal(t, []string{"bootstrap", "  kept as is", "--logger-type", "json", "--ssh-user", "json", "--x=@" + inner, "--", "@" + inner}, got)

	// Not nil: cobra given nil args reads os.Args.
	got, err = expandArgFiles([]string{"@" + write("empty", "# nothing\n")})
	require.NoError(t, err)
	require.Equal(t, []string{}, got)

	missing := filepath.Join(dir, "missing")

	long := write("long", strings.Repeat("x", 70000)+"\n")
	for arg, want := range map[string]string{
		"@":           "expected @ file to expand arguments from",
		"@" + missing: fmt.Sprintf("failed to open arguments file %q: open %s: no such file or directory", missing, missing),
		"@" + dir:     fmt.Sprintf("failed to read arguments from %q: read %s: is a directory", dir, dir),
		"@" + long:    fmt.Sprintf("failed to read arguments from %q: bufio.Scanner: token too long", long),
	} {
		_, err := expandArgFiles([]string{"bootstrap", arg})
		require.EqualError(t, err, want)
	}
}

// cobra parses the expanded arguments; a file with none leaves a bare dhctl, which prints usage.
func TestExecuteExpandsArgFiles(t *testing.T) {
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	require.NoError(t, os.WriteFile(args, []byte("bootstrap\n--config\nx\n--resources-timeout\n20m\n"), 0o600))

	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))

	opts := options.New()
	root := buildRoot(commandList, opts, nil, stopAtPrepare)
	_, err := execute(t.Context(), root, []string{"@" + args})
	require.ErrorIs(t, err, errPrepared)
	require.Equal(t, 20*time.Minute, opts.Bootstrap.ResourcesTimeout)

	root = buildRoot(commandList, options.New(), nil, func(*cobra.Command) error { return errPrepared })

	var out bytes.Buffer
	root.SetOut(&out)
	_, err = execute(t.Context(), root, []string{"@" + empty})
	require.NoError(t, err)
	require.Contains(t, out.String(), "Available Commands:")
}

func TestIsAllowed(t *testing.T) {
	cases := []struct {
		top, name string
		allowed   []string
		want      bool
	}{
		{"bootstrap", "bootstrap", nil, true},
		{"bootstrap", "bootstrap", []string{"bootstrap", "converge"}, true},
		{"destroy", "destroy", []string{"bootstrap", "converge"}, false},
		{"terraform", "check", []string{"terraform"}, true},
		{"terraform", "check", []string{"terraform *"}, true},
		{"terraform", "terraform", []string{"terraform check"}, true},
		{"terraform", "check", []string{"terraform converge-exporter check"}, true},
		{"terraform", "converge", []string{"terraform converge-exporter check"}, false},
		{"terraform", "check", []string{"converge-periodical", "terraform check"}, true},
		{"converge", "converge", []string{"converge-periodical", "terraform check"}, false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, isAllowed(tc.top, tc.name, tc.allowed), "%s %s in %q", tc.top, tc.name, tc.allowed)
	}
}

func TestBuildRoot(t *testing.T) {
	define := func(*cobra.Command, *options.Options) {}
	tree := []Command{
		{Name: "bootstrap", Help: "Bootstrap cluster", DefineFunc: define, Subcommands: []Command{
			{Name: "install", Help: "Install component", DefineFunc: define},
		}},
		{Name: "converge", Help: "Converge cluster", DefineFunc: define},
		{Name: "test", Help: "Tests", Subcommands: []Command{
			{Name: "control-plane", Help: "Control plane", Subcommands: []Command{
				{Name: "manager", Help: "Manager", DefineFunc: define},
			}},
		}},
	}

	cases := []struct {
		allowed []string
		want    []string
	}{
		{nil, []string{"bootstrap", "bootstrap install", "converge", "test", "test control-plane", "test control-plane manager"}},
		{[]string{"bootstrap"}, []string{"bootstrap", "bootstrap install"}},
		{[]string{"converge", "test *"}, []string{"converge", "test", "test control-plane", "test control-plane manager"}},
		// A child of a filtered-out parent is left out with it.
		{[]string{"test manager"}, []string{"test"}},
	}
	for _, tc := range cases {
		root := buildRoot(tree, options.New(), tc.allowed, func(*cobra.Command) error { return nil })
		require.Equal(t, tc.want, commandPaths(root), "%q", tc.allowed)
	}
}

func commandPaths(root *cobra.Command) []string {
	var (
		out  []string
		walk func(c *cobra.Command)
	)

	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden {
				continue
			}

			out = append(out, strings.TrimPrefix(sub.CommandPath(), root.Name()+" "))
			walk(sub)
		}
	}
	walk(root)

	return out
}

// DHCTL_CLI_ALLOWED_COMMANDS lets through the config edit commands it lists and no other.
func TestAllowedCommandsFilterConfigEdit(t *testing.T) {
	root := buildRoot(commandList, options.New(), []string{"config edit cluster-configuration"}, stopAtPrepare)
	require.Equal(t, []string{"config", "config edit", "config edit cluster-configuration"}, commandPaths(root))
}
