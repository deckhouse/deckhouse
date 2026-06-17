// Copyright 2021 Flant JSC
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

package helpers

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	changeregistry "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/change_registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/jwt"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func DefineHelperCommands(rootCmd *cobra.Command, logger *log.Logger) {
	helpersCmd := &cobra.Command{
		Use:   "helper",
		Short: "Deckhouse helpers.",
	}
	rootCmd.AddCommand(helpersCmd)

	{
		var (
			privateKeyPath string
			claims         map[string]string
			ttl            time.Duration
		)

		genJWTCmd := &cobra.Command{
			Use:   "gen-jwt",
			Short: "Generate JWT token.",
			RunE: func(_ *cobra.Command, _ []string) error {
				if _, err := os.Stat(privateKeyPath); err != nil {
					return fmt.Errorf("private-key-path: %w", err)
				}
				return jwt.GenJWT(privateKeyPath, claims, ttl)
			},
		}
		genJWTCmd.Flags().StringVar(&privateKeyPath, "private-key-path", "", "Path to private RSA key in PEM format.")
		genJWTCmd.Flags().StringToStringVar(&claims, "claim", nil, "Claims for token (ex --claim iss=deckhouse --claim sub=akakiy).")
		genJWTCmd.Flags().DurationVar(&ttl, "ttl", 0, "TTL duration (ex. 10s).")
		_ = genJWTCmd.MarkFlagRequired("private-key-path")
		_ = genJWTCmd.MarkFlagRequired("claim")
		_ = genJWTCmd.MarkFlagRequired("ttl")
		helpersCmd.AddCommand(genJWTCmd)
	}

	{
		var (
			user        string
			password    string
			caFile      string
			scheme      string
			dryRun      bool
			newImageTag string
		)

		changeRegistryCmd := &cobra.Command{
			Use:   "change-registry NEW_REGISTRY",
			Short: "Change registry for deckhouse images.",
			Long: "Change registry for deckhouse images.\n\n" +
				"NEW_REGISTRY: Registry that will be used for deckhouse images " +
				"(example: registry.deckhouse.io/deckhouse/ce). By default, https will be used; " +
				"pass --scheme=http to switch to http.",
			Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				if caFile != "" {
					if _, err := os.Stat(caFile); err != nil {
						return fmt.Errorf("ca-file: %w", err)
					}
				}
				return changeregistry.ChangeRegistry(args[0], user, password, caFile, newImageTag, scheme, dryRun, logger)
			},
		}
		changeRegistryCmd.Flags().StringVar(&user, "user", "", "User with pull access to registry.")
		changeRegistryCmd.Flags().StringVar(&password, "password", "", "Password/token for registry user.")
		changeRegistryCmd.Flags().StringVar(&caFile, "ca-file", "", "Path to registry CA.")
		changeRegistryCmd.Flags().StringVar(&scheme, "scheme", "", `Used scheme while connecting to registry, "http" or "https".`)
		changeRegistryCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't change deckhouse resources, only print them.")
		changeRegistryCmd.Flags().StringVar(&newImageTag, "new-deckhouse-tag", "", "New tag that will be used for deckhouse deployment image (by default current tag from deckhouse deployment will be used).")
		helpersCmd.AddCommand(changeRegistryCmd)
	}
}
