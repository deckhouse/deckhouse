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
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// allowedCommandsFromEnv is DHCTL_CLI_ALLOWED_COMMANDS split on ", "; nil allows every command.
func allowedCommandsFromEnv() []string {
	commands := os.Getenv("DHCTL_CLI_ALLOWED_COMMANDS")
	if commands == "" {
		return nil
	}

	return strings.Split(commands, ", ")
}

type Command struct {
	Name        string
	Help        string
	DefineFunc  func(cmd *cobra.Command, opts *options.Options)
	Subcommands []Command
}

// buildRoot builds the cobra commands of tree. A command of the tree applies its flags first, then
// runs their hooks, then prepare; cobra's help and completion and a bare group run without prepare.
func buildRoot(tree []Command, opts *options.Options, allowed []string, prepare func(*cobra.Command) error) *cobra.Command {
	cobra.EnableCommandSorting = false // help lists commands as the tree declares them
	root := &cobra.Command{
		Use:           app.AppName,
		Short:         appHelp,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetGlobalNormalizationFunc(app.KingpinFlagNames)
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if errors.Is(err, errShowVersion) {
			_, err := fmt.Fprintln(cmd.ErrOrStderr(), opts.BuildInfo.AppVersion)
			return err
		}

		return app.FlagError(cmd, err)
	})
	app.ShowEnvInHelp(root)
	version := root.PersistentFlags().VarPF(versionValue{}, "version", "", "Show application version.")
	version.NoOptDefVal = "true"

	app.GlobalFlags(root, &opts.Global)

	for _, c := range tree {
		if !isAllowed(c.Name, c.Name, allowed) {
			continue
		}

		cmd := buildCommand(c, c.Name, opts, allowed, prepare)
		cmd.PersistentPreRunE = applyFlagsHook(opts)
		root.AddCommand(cmd)
	}

	return root
}

// buildCommand builds c with the subcommands DHCTL_CLI_ALLOWED_COMMANDS lets through under top.
func buildCommand(c Command, top string, opts *options.Options, allowed []string, prepare func(*cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{Use: c.Name, Short: c.Help, Args: cobra.NoArgs}
	if c.DefineFunc != nil {
		c.DefineFunc(cmd, opts)
	}

	for _, sub := range c.Subcommands {
		if isAllowed(top, sub.Name, allowed) {
			cmd.AddCommand(buildCommand(sub, top, opts, allowed, prepare))
		}
	}

	if cmd.HasSubCommands() {
		cmd.RunE = requireSubcommand
		return cmd
	}
	// After the flag hooks DefineFunc added: a flag they refuse stops the run before prepare
	// sets up the tmp dir, as kingpin's PreActions did.
	app.AfterApply(cmd, prepare)

	return cmd
}

// requireSubcommand makes a group runnable, so cobra checks NoArgs and refuses a mistyped
// subcommand. A bare group fails with kingpin's error and exit code, as dhctl did before cobra.
func requireSubcommand(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("must select a subcommand of '%s'", commandName(cmd))
}

func applyFlagsHook(opts *options.Options) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		return applyFlags(cmd, opts)
	}
}

// isAllowed reads DHCTL_CLI_ALLOWED_COMMANDS: "name" and "name *" allow the whole tree of a
// top-level command, "name sub1 sub2" allows the listed subcommands under it when their groups
// are listed too.
func isAllowed(top, name string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}

	for _, entry := range allowed {
		words := strings.Fields(entry)
		if len(words) == 0 || words[0] != top {
			continue
		}

		if name == top || len(words) == 1 {
			return true
		}

		if words[1] == "*" || slices.Contains(words[1:], name) {
			return true
		}
	}

	return false
}

// errShowVersion stops parsing at --version: kingpin printed the version to stderr and exited
// before it checked required flags or ran anything.
var errShowVersion = errors.New("show version")

// versionValue is kingpin's --version, which every command took.
type versionValue struct{}

var _ pflag.Value = versionValue{}

func (versionValue) String() string   { return "false" }
func (versionValue) Type() string     { return app.FlagTypeBool }
func (versionValue) Set(string) error { return errShowVersion }

// execute runs root on args with kingpin's `@file` arguments expanded.
func execute(ctx context.Context, root *cobra.Command, args []string) (*cobra.Command, error) {
	expanded, err := expandArgFiles(args)
	if err != nil {
		return root, err
	}

	root.SetArgs(expanded)

	return root.ExecuteContextC(ctx)
}

// expandArgFiles puts the lines of the file in place of an `@file` argument, `@file` lines
// expanded in turn; nothing after `--` is. Like alecthomas/kingpin parser.go ParseContext.Next.
func expandArgFiles(args []string) ([]string, error) {
	out := make([]string, 0, len(args))
	for len(args) > 0 {
		arg := args[0]

		args = args[1:]
		if arg == "--" {
			return append(append(out, arg), args...), nil
		}

		if !strings.HasPrefix(arg, "@") {
			out = append(out, arg)
			continue
		}

		lines, err := readArgFile(arg[1:])
		if err != nil {
			return nil, err
		}

		args = append(lines, args...)
	}

	return out, nil
}

// readArgFile takes a line as one argument and skips blank lines and lines starting with #; the
// error texts are kingpin's. Mirrors alecthomas/kingpin parser.go ExpandArgsFromFile.
func readArgFile(name string) ([]string, error) {
	if name == "" {
		return nil, errors.New("expected @ file to expand arguments from")
	}

	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open arguments file %q: %w", name, err)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read arguments from %q: %w", name, err)
	}

	return lines, nil
}
