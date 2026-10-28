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
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
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
		var output string

		convergeCmd := &cobra.Command{
			Use:   "converge",
			Short: "Show the modules convergence waits for and their errors.",
			RunE: func(_ *cobra.Command, _ []string) error {
				switch output {
				case "table":
					return printConverge()
				case "json", "yaml":
					return printResponse(outputURL("api/v1/converge", "", output))
				default:
					return fmt.Errorf("unknown output %q: table, json or yaml", output)
				}
			},
		}
		convergeCmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json or yaml.")
		definePackagesDebugSocketFlag(convergeCmd)
		packagesCmd.AddCommand(convergeCmd)
	}

	{
		queueCmd := &cobra.Command{Use: "queue", Short: "Hook lane operations."}
		packagesCmd.AddCommand(queueCmd)

		var packageName, output string

		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump the hook lanes of every package, or of one.",
			RunE: func(_ *cobra.Command, _ []string) error {
				switch output {
				case "table":
					return printLanes(packageName)
				case "json", "yaml":
					return printResponse(outputURL("api/v1/queues/dump", packageName, output))
				default:
					return fmt.Errorf("unknown output %q: table, json or yaml", output)
				}
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		dumpCmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json or yaml.")
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
	if v, ok := os.LookupEnv(envconfig.EnvPackagesDebugUnixSocket); ok && v != "" {
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

// outputURL is path in format, narrowed to name when it is set.
func outputURL(path, name, format string) string {
	u := path + "?output=" + format
	if name != "" {
		u += "&" + url.Values{"name": {name}}.Encode()
	}

	return u
}

// printLanes prints the hook lanes as a table, one row per lane.
func printLanes(name string) error {
	client, err := newSocketClient(packagesDebugSocket)
	if err != nil {
		return err
	}
	defer client.Close()

	out, err := client.Get(context.Background(), outputURL("api/v1/queues/dump", name, "json"))
	if err != nil {
		return err
	}

	var dump hooklane.Dump
	if err := json.Unmarshal(out, &dump); err != nil {
		return fmt.Errorf("decode the lanes: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PACKAGE\tQUEUE\tSTATE\tINFLIGHT\tRUNNING-FOR\tWAITING\tOLDEST\tFAILURE\tNEXT-RETRY")

	for _, owner := range slices.Sorted(maps.Keys(dump.Lanes)) {
		lanes := dump.Lanes[owner]
		for _, queue := range slices.Sorted(maps.Keys(lanes)) {
			l := lanes[queue]

			inFlight, runningFor := "-", "-"
			if l.InFlight != nil {
				inFlight, runningFor = l.InFlight.Hook, l.InFlight.RunningFor
			}

			failure, nextRetry := "-", "-"
			if l.Failure != nil {
				failure, nextRetry = l.Failure.Hook+": "+l.Failure.Error, l.Failure.NextRetry
			}

			oldest := l.Waiting.OldestAge
			if oldest == "" {
				oldest = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
				owner, queue, l.State, inFlight, runningFor, l.Waiting.Count, oldest, failure, nextRetry)
		}
	}

	return w.Flush()
}

// printConverge prints Converged, or one row per node the convergence waits for.
func printConverge() error {
	client, err := newSocketClient(packagesDebugSocket)
	if err != nil {
		return err
	}
	defer client.Close()

	out, err := client.Get(context.Background(), outputURL("api/v1/converge", "", "json"))
	if err != nil {
		return err
	}

	var dump engine.ConvergeDump
	if err := json.Unmarshal(out, &dump); err != nil {
		return fmt.Errorf("decode the convergence: %w", err)
	}

	if dump.Converged {
		fmt.Println("Converged")

		return nil
	}

	if dump.Paused {
		fmt.Println("Not started: the scheduler waits for the boot gate")
	}

	if len(dump.Packages) == 0 {
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PACKAGE\tVERSION\tORDER\tSTATE\tWAITS-FOR\tERROR")

	for _, name := range slices.Sorted(maps.Keys(dump.Packages)) {
		p := dump.Packages[name]

		version, state, waitsFor, failure := "-", "running", "-", "-"
		if p.Version != "" {
			version = p.Version
		}

		if p.WaitsFor != "" {
			state, waitsFor = "waiting", p.WaitsFor
		}

		if p.Error != "" {
			state, failure = "error", p.Error
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", name, version, p.Order, state, waitsFor, failure)
	}

	return w.Flush()
}

// namedURL narrows a dump to a single package; an empty name means every package.
func namedURL(path, name string) string {
	if name == "" {
		return yamlURL(path)
	}

	return yamlURL(path) + "&" + url.Values{"name": {name}}.Encode()
}
