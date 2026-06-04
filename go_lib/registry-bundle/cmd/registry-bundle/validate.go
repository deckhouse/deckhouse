/*
Copyright 2026 Flant JSC

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

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/bundle"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
)

func newValidateCmd(logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <bundle-path>",
		Short: "Validate bundle archives structure",
		Long:  `Check if all archives in the directory contain valid OCI layouts with unique repository paths.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			bundlePath := args[0]
			bndl, err := bundle.New(
				ctx,
				logger,
				bundlePath,
			)
			if err != nil {
				return fmt.Errorf("load bundle from %q: %w", bundlePath, err)
			}

			defer func() {
				if err := bndl.Close(); err != nil {
					logger.Errorf("close bundle error: %s", err.Error())
				}
			}()

			logger.Infof("no errors")
			return nil
		},
	}

	return cmd
}
