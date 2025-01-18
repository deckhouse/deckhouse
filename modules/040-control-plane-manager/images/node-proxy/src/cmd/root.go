/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"node-proxy-sidecar/internal/k8s"
)

var (
	Version    string
	devMode    bool
	socketPath string
)

var rootCmd = &cobra.Command{
	Use:   "node-proxy-sidecar",
	Short: "Node Proxy Sidecar",
	Run:   run,
}

func init() {
	rootCmd.Flags().BoolVar(&devMode, "dev", false, "local development")
	rootCmd.Flags().StringVar(&socketPath, "socket-path", "", "path to haproxy socket")
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func test(updatedList []string) {
	fmt.Println("Updated Endpoints List:", updatedList)
}

func run(cmd *cobra.Command, args []string) {
	k8sClient := k8s.NewClient(devMode)

	err := k8sClient.WatchEndpoints("default", "kubernetes", []string{"https"}, test)
	if err != nil {
		fmt.Println("Error watching endpoints:", err)
		return
	}
	select {}
}
