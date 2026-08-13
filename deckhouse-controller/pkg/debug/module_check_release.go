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

package debug

import (
	"fmt"
	"path/filepath"

	"github.com/deckhouse/d8sql"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releasegates"
)

// DefineModuleCheckReleaseCommand attaches `check-release` to the `module`
// command: a dry run of the release gates the module release controller applies
// between download and install, against the current cluster.
func DefineModuleCheckReleaseCommand(rootCmd *cobra.Command) {
	moduleCmd := findSubcommand(rootCmd, "module")
	if moduleCmd == nil {
		moduleCmd = &cobra.Command{Use: "module", Short: "Manage modules."}
		rootCmd.AddCommand(moduleCmd)
	}

	var (
		kubeconfig        string
		migrations        string
		deckhouseVersion  string
		edition           string
		bundle            string
		kubernetesVersion string
	)

	cmd := &cobra.Command{
		Use:   "check-release MODULE_DIR",
		Short: "Run the release gates (release/validations, release/migrations) of a module directory against the cluster.",
		Args:  cobra.ExactArgs(1),
		// a failing gate is the command doing its job, not a usage error
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clientConfig(kubeconfig)
			if err != nil {
				return err
			}

			platform, err := platformValues(cfg, deckhouseVersion, edition, bundle, kubernetesVersion)
			if err != nil {
				return err
			}

			engine, err := d8sql.NewForConfig(cfg, platform.Option())
			if err != nil {
				return fmt.Errorf("create engine: %w", err)
			}

			files, err := releasegates.Validations(args[0])
			if err != nil {
				return err
			}

			pending, err := migrationFiles(args[0], migrations)
			if err != nil {
				return err
			}

			for _, migration := range pending {
				files = append(files, migration.Path)
			}

			if len(files) == 0 {
				cmd.Printf("no release gates found in %s\n", args[0])

				return nil
			}

			for _, file := range files {
				changed, err := releasegates.Run(cmd.Context(), engine, file)
				if err != nil {
					// the error already names the file it came from
					cmd.Printf("FAIL %v\n", err)

					return fmt.Errorf("release gates failed")
				}

				cmd.Printf("OK   %s (objects changed: %d)\n", filepath.Base(file), changed)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to the kubeconfig file. Defaults to $KUBECONFIG or the in-cluster config.")
	cmd.Flags().StringVar(&migrations, "migrations", "none", "Which migrations to run besides the validations: none, up or down.")
	cmd.Flags().StringVar(&deckhouseVersion, "deckhouse-version", "", "Override the deckhouseVersion value of the v_d8_platform table.")
	cmd.Flags().StringVar(&edition, "edition", "", "Override the deckhouseEdition value of the v_d8_platform table.")
	cmd.Flags().StringVar(&bundle, "bundle", "", "Override the deckhouseBundle value of the v_d8_platform table.")
	cmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", "", "Override the kubernetesVersion value of the v_d8_platform table.")

	moduleCmd.AddCommand(cmd)
}

func clientConfig(kubeconfig string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("client config: %w", err)
	}

	return cfg, nil
}

// platformValues fills the v_d8_platform row the same way the controller does,
// falling back to explicit flags for the sources unavailable outside the
// deckhouse pod.
func platformValues(cfg *rest.Config, deckhouseVersion, edition, bundle, kubernetesVersion string) (releasegates.Platform, error) {
	platform := releasegates.Platform{
		DeckhouseVersion:  deckhouseVersion,
		DeckhouseEdition:  edition,
		DeckhouseBundle:   bundle,
		KubernetesVersion: kubernetesVersion,
	}

	// the edition file only exists inside the deckhouse image
	if parsed, err := d8edition.Parse(app.Version); err == nil {
		if platform.DeckhouseVersion == "" {
			platform.DeckhouseVersion = parsed.Version
		}
		if platform.DeckhouseEdition == "" {
			platform.DeckhouseEdition = parsed.Name
		}
		if platform.DeckhouseBundle == "" {
			platform.DeckhouseBundle = parsed.Bundle
		}
	}

	if platform.KubernetesVersion == "" {
		client, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return platform, fmt.Errorf("create kubernetes client: %w", err)
		}

		version, err := client.Discovery().ServerVersion()
		if err != nil {
			return platform, fmt.Errorf("discover kubernetes version: %w", err)
		}

		platform.KubernetesVersion = releasegates.NormalizeVersion(version.GitVersion)
	}

	return platform, nil
}

// migrationFiles selects the migrations to run: everything ascending for up,
// everything descending for down. The dry run has no applied-migrations state,
// so it always replays the module's full set.
func migrationFiles(modulePath, mode string) ([]releasegates.Migration, error) {
	if mode == "none" || mode == "" {
		return nil, nil
	}

	migrations, err := releasegates.Migrations(modulePath)
	if err != nil {
		return nil, err
	}

	switch mode {
	case "up":
		return releasegates.PendingUp(migrations, 0), nil
	case "down":
		return releasegates.PendingDown(migrations, 0), nil
	default:
		return nil, fmt.Errorf("unknown --migrations value %q, expected none, up or down", mode)
	}
}
