// Copyright 2025 Flant JSC
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

package debug

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var packagesDebugSocket = "/tmp/deckhouse-debug.socket"

func DefinePackagesCommands(rootCmd *cobra.Command) {
	packagesCmd := &cobra.Command{
		Use:   "packages",
		Short: "Package debug commands.",
	}
	rootCmd.AddCommand(packagesCmd)

	{
		var packageName string

		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all/specific packages state from memory.",
			RunE: func(_ *cobra.Command, _ []string) error {
				return printResponse(namedURL("api/v1/packages/dump", packageName))
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		packagesCmd.AddCommand(dumpCmd)
	}

	{
		globalCmd := &cobra.Command{Use: "global", Short: "Global module operations."}
		packagesCmd.AddCommand(globalCmd)

		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump the global module state from memory.",
			RunE: func(_ *cobra.Command, _ []string) error {
				return printResponse(yamlURL("api/v1/packages/global/dump"))
			},
		}
		definePackagesDebugSocketFlag(dumpCmd)
		globalCmd.AddCommand(dumpCmd)
	}

	{
		schedulerCmd := &cobra.Command{Use: "scheduler", Short: "Scheduler operations."}
		packagesCmd.AddCommand(schedulerCmd)

		var packageName string

		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all scheduler node state from memory.",
			RunE: func(_ *cobra.Command, _ []string) error {
				return printResponse(namedURL("api/v1/scheduler/dump", packageName))
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		schedulerCmd.AddCommand(dumpCmd)
	}

	{
		queueCmd := &cobra.Command{Use: "queue", Short: "Queue operations."}
		packagesCmd.AddCommand(queueCmd)

		var packageName string

		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all package queues with tasks.",
			RunE: func(_ *cobra.Command, _ []string) error {
				return printResponse(namedURL("api/v1/queues/dump", packageName))
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		queueCmd.AddCommand(dumpCmd)
	}

	{
		renderCmd := &cobra.Command{
			Use:   "render PACKAGE_NAME",
			Short: "Render package Helm templates.",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				// Rendered manifests are YAML already, so this route serves no other format.
				return printResponse("api/v1/packages/render/" + args[0])
			},
		}
		definePackagesDebugSocketFlag(renderCmd)
		packagesCmd.AddCommand(renderCmd)
	}

	{
		snapshotsCmd := &cobra.Command{
			Use:   "snapshots PACKAGE_NAME",
			Short: "Dump hook snapshots for a package.",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				return printResponse(yamlURL("api/v1/packages/snapshots/" + args[0]))
			},
		}
		definePackagesDebugSocketFlag(snapshotsCmd)
		packagesCmd.AddCommand(snapshotsCmd)
	}
}

func definePackagesDebugSocketFlag(cmd *cobra.Command) {
	defaultSocket := packagesDebugSocket
	if v, ok := os.LookupEnv("PACKAGES_DEBUG_UNIX_SOCKET"); ok && v != "" {
		defaultSocket = v
		packagesDebugSocket = v
	}
	cmd.Flags().StringVar(&packagesDebugSocket, "debug-unix-socket", defaultSocket, "Path to Unix socket for packages debug endpoint.")
}

// printResponse requests path over the debug socket and prints what comes back.
func printResponse(path string) error {
	client, err := newSocketClient(packagesDebugSocket)
	if err != nil {
		return err
	}
	defer client.Close()

	out, err := client.Get(context.Background(), path)
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}

// yamlURL asks the API for YAML: the CLI prints to a terminal, while the API
// answers JSON by default.
func yamlURL(path string) string {
	return path + "?output=yaml"
}

// namedURL narrows a dump to a single package; an empty name means every package.
func namedURL(path, name string) string {
	if name == "" {
		return yamlURL(path)
	}

	return yamlURL(path) + "&" + url.Values{"name": {name}}.Encode()
}
