// Copyright 2022 Flant JSC
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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/deckhouse"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var semVerRegex = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?$`)

const (
	UnknownReleaseChannelSecretDiscovery = "Unknown"
	ReleaseChannelAuto                   = "auto"

	ReleaseChannelAlpha       = "alpha"
	ReleaseChannelBeta        = "beta"
	ReleaseChannelStable      = "stable"
	ReleaseChannelEarlyAccess = "early-access"
	ReleaseChannelRockSolid   = "rock-solid"
)

func DefineRegistryCommand(rootCmd *cobra.Command, logger *log.Logger) {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Deckhouse repository work.",
	}
	rootCmd.AddCommand(registryCmd)

	registryGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get from registry.",
	}
	registryCmd.AddCommand(registryGetCmd)

	registerReleaseCommand(registryGetCmd, logger)
	registerSourceCommand(registryGetCmd)
	registerModuleCommand(registryGetCmd, logger)
}

func registerReleaseCommand(parent *cobra.Command, logger *log.Logger) {
	var (
		releaseChannel string
		all            bool
	)

	releasesCmd := &cobra.Command{
		Use:     "releases",
		Aliases: []string{"release", "rel"},
		Short:   "Release resource. Aliases: 'release','rel'",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.TODO()

			registry, channel, rconf, err := getDeckhouseRegistry(ctx)
			if err != nil {
				return fmt.Errorf("get deckhouse registry: %w", err)
			}

			reg, err := deckhouseRegistry(registry, rconf, logger)
			if err != nil {
				return fmt.Errorf("open deckhouse registry: %w", err)
			}

			if releaseChannel != "" {
				if releaseChannel != ReleaseChannelAuto {
					channel = releaseChannel
				}

				if channel == "" || channel == UnknownReleaseChannelSecretDiscovery {
					channel = ReleaseChannelStable
				}

				return handleGetDeckhouseRelease(ctx, reg.Deckhouse().Releases(), channel, all)
			}

			return handleListDeckhouseReleases(ctx, reg.Deckhouse().BasicService, all)
		},
	}
	releasesCmd.Flags().StringVarP(&releaseChannel, "channel", "c",
		"",
		"Release channel."+
			" If release is 'auto' - using default channel from configuration."+
			" If there is not default channel in configuration - use 'stable'."+
			fmt.Sprintf(" Allowed: %s, %s, %s, %s, %s, %s.",
				ReleaseChannelAlpha, ReleaseChannelBeta, ReleaseChannelStable,
				ReleaseChannelEarlyAccess, ReleaseChannelRockSolid, ReleaseChannelAuto))
	releasesCmd.Flags().BoolVar(&all, "all", false, "Output without restrictions.")
	releasesCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateEnumFlag(cmd, "channel", releaseChannel,
			ReleaseChannelAlpha, ReleaseChannelBeta, ReleaseChannelStable,
			ReleaseChannelEarlyAccess, ReleaseChannelRockSolid, ReleaseChannelAuto)
	}
	parent.AddCommand(releasesCmd)
}

func handleListDeckhouseReleases(ctx context.Context, svc *service.BasicService, all bool) error {
	ls, err := svc.ListTags(ctx)
	if err != nil {
		return fmt.Errorf("list deckhouse releases: %w", err)
	}

	// if we need full tags list, not only semVer
	if !all {
		res := make([]string, 0, 1)
		for _, v := range ls {
			if semVerRegex.MatchString(v) {
				res = append(res, v)
			}
		}

		ls = res
	}

	if len(ls) == 0 {
		if all {
			fmt.Println("Releases not found")
		} else {
			fmt.Println("Releases with semVer not found. Use --all argument to watch all releases in the registry")
		}

		return nil
	}

	fmt.Println(strings.Join(ls, "\n"))

	return nil
}

func handleGetDeckhouseRelease(ctx context.Context, svc *deckhouse.ReleaseService, channel string, all bool) error {
	rel, err := svc.Fetch(ctx, strcase.ToKebab(channel))
	if err != nil {
		if errors.Is(err, dhregistry.ErrImageNotFound) {
			return fmt.Errorf("deckhouse release with channel '%s' is not found", channel)
		}

		return fmt.Errorf("get deckhouse release: %w", err)
	}

	meta, err := rel.Metadata()
	if err != nil {
		return fmt.Errorf("get deckhouse release: %w", err)
	}

	if meta.Version == "" {
		return errors.New("release metadata malformed: no version found")
	}

	if !all {
		fmt.Printf("Deckhouse version in channel '%s': %s\n", channel, meta.Version)

		return nil
	}

	// Raw is the version.json the release image carries, printed as-is so the
	// output is whatever the registry holds rather than a re-encoding of it.
	return printJSON(meta.Raw)
}

// printJSON writes raw JSON re-indented for a terminal.
func printJSON(raw []byte) error {
	buffer := &bytes.Buffer{}
	if err := json.Indent(buffer, raw, "", "    "); err != nil {
		return fmt.Errorf("indent json: %w", err)
	}

	fmt.Printf("%s\n", buffer.String())

	return nil
}

func registerSourceCommand(parent *cobra.Command) {
	sourcesCmd := &cobra.Command{
		Use:     "sources",
		Aliases: []string{"source", "src"},
		Short:   "Source resources. Aliases: 'source','src'",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.TODO()

			k8sClient, err := newKubernetesClient()
			if err != nil {
				panic(err)
			}

			msl := new(v1alpha1.ModuleSourceList)
			if err := k8sClient.List(ctx, msl); err != nil {
				return fmt.Errorf("list ModuleSource: %w", err)
			}

			srcs := make([]string, 0, len(msl.Items))
			for _, ms := range msl.Items {
				srcs = append(srcs, ms.GetName())
			}

			fmt.Printf("Module sources found (%d):\n\n", len(srcs))

			for _, src := range srcs {
				fmt.Printf("%s\n", src)
			}

			return nil
		},
	}
	parent.AddCommand(sourcesCmd)
}

func registerModuleCommand(parent *cobra.Command, logger *log.Logger) {
	var (
		moduleChannel string
		all           bool
	)

	// deckhouse-controller registry get modules <module-source> [<module-name>]
	modulesCmd := &cobra.Command{
		Use:     "modules MODULE_SOURCE [MODULE_NAME]",
		Aliases: []string{"module", "mod"},
		Short:   "Show modules list. Aliases: 'module','mod'",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.TODO()

			moduleSource := args[0]
			moduleName := ""
			if len(args) == 2 {
				moduleName = args[1]
			}

			registry, rconf, err := getModuleRegistry(ctx, moduleSource)
			if err != nil {
				return fmt.Errorf("get module registry: %w", err)
			}

			catalog, err := moduleCatalog(registry, rconf, logger)
			if err != nil {
				return fmt.Errorf("open module catalog: %w", err)
			}

			if moduleName != "" {
				if moduleChannel != "" {
					return handleGetModuleInfoInChannel(ctx, catalog, moduleName, moduleChannel, all)
				}

				return handleListModulesVersions(ctx, catalog, moduleName, all)
			}

			return handleListModulesNames(ctx, catalog, all)
		},
	}
	modulesCmd.Flags().StringVarP(&moduleChannel, "channel", "c", "",
		"Release channel."+
			fmt.Sprintf(" Allowed: %s, %s, %s, %s, %s.",
				ReleaseChannelAlpha, ReleaseChannelBeta, ReleaseChannelStable,
				ReleaseChannelEarlyAccess, ReleaseChannelRockSolid))
	modulesCmd.Flags().BoolVar(&all, "all", false, "Complete list of tags.")
	modulesCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateEnumFlag(cmd, "channel", moduleChannel,
			ReleaseChannelAlpha, ReleaseChannelBeta, ReleaseChannelStable,
			ReleaseChannelEarlyAccess, ReleaseChannelRockSolid)
	}
	parent.AddCommand(modulesCmd)
}

// validateEnumFlag returns an error when the named flag is set to a value
// outside the allowed set. Empty values are treated as "unset" and skip the
// check, matching kingpin's behavior for optional Enum flags.
func validateEnumFlag(_ *cobra.Command, name, value string, allowed ...string) error { //nolint:unparam // general helper; only the channel flag uses it today
	if value == "" {
		return nil
	}
	for _, v := range allowed {
		if v == value {
			return nil
		}
	}
	return fmt.Errorf("flag --%s must be one of: %s", name, strings.Join(allowed, ", "))
}

func handleGetModuleInfoInChannel(ctx context.Context, catalog *module.Catalog, name string, channel string, all bool) error {
	// One pull: the release image is fetched once and both the version and the
	// manifest are read from the snapshot.
	rel, err := catalog.Module(name).Releases().Fetch(ctx, strcase.ToKebab(channel))
	if err != nil {
		if errors.Is(err, dhregistry.ErrImageNotFound) {
			return fmt.Errorf("module release with name '%s' and channel '%s' is not found", name, channel)
		}

		return fmt.Errorf("get module release %s: %w", name, err)
	}

	version, err := rel.Version()
	if err != nil {
		if errors.Is(err, dhregistry.ErrNoVersionMetadata) {
			return fmt.Errorf("module release %q metadata malformed: no version found", name)
		}

		return fmt.Errorf("get module release %s: %w", name, err)
	}

	if !all {
		fmt.Printf("Module version in channel '%s': %s\n", channel, version)

		return nil
	}

	info := struct {
		Version    string             `json:"version"`
		Definition *definition.Module `json:"module,omitempty"`
	}{Version: version}

	// The manifest is optional: older releases ship none and it has to be read
	// from the module image instead.
	def, err := rel.Definition()
	if err != nil && !errors.Is(err, dhregistry.ErrFileNotFound) {
		return fmt.Errorf("get module definition: %w", err)
	}

	info.Definition = def

	raw, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal module release: %w", err)
	}

	return printJSON(raw)
}

func handleListModulesVersions(ctx context.Context, catalog *module.Catalog, name string, all bool) error {
	ls, err := catalog.Module(name).ListTags(ctx)
	if err != nil {
		if errors.Is(err, dhregistry.ErrImageNotFound) {
			return fmt.Errorf("module release with name '%s' is not found", name)
		}

		return fmt.Errorf("list module tags: %w", err)
	}

	// if we need full tags list, not only semVer
	if !all {
		res := make([]string, 0, 1)
		for _, v := range ls {
			if semVerRegex.MatchString(v) {
				res = append(res, v)
			}
		}

		ls = res
	}

	if len(ls) == 0 {
		if all {
			fmt.Println("Module releases not found")
		} else {
			fmt.Println("Module releases with semVer not found. Use --all argument to watch all releases in the registry")
		}

		return nil
	}

	fmt.Println(strings.Join(ls, "\n"))

	return nil
}

func handleListModulesNames(ctx context.Context, catalog *module.Catalog, all bool) error {
	modules, err := catalog.List(ctx)
	if err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	if len(modules) == 0 {
		if all {
			fmt.Println("Modules not found")
		} else {
			fmt.Println("Modules with semVer not found. Use --all argument to watch all releases in the registry")
		}

		return nil
	}

	fmt.Printf("Modules found (%d):\n\n", len(modules))

	fmt.Println(strings.Join(modules, "\n"))

	return nil
}

func newKubernetesClient() (client.Client, error) {
	scheme := runtime.NewScheme()

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	restConfig := ctrl.GetConfigOrDie()
	opts := client.Options{
		Scheme: scheme,
	}

	k8sClient, err := client.New(restConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return k8sClient, nil
}

func getDeckhouseRegistry(ctx context.Context) (string, string, *utils.RegistryConfig, error) {
	k8sClient, err := newKubernetesClient()
	if err != nil {
		panic(err)
	}

	secret := new(corev1.Secret)
	registryKey := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: app.SecretRegistry}
	if err := k8sClient.Get(ctx, registryKey, secret); err != nil {
		return "", "", nil, fmt.Errorf("get registry secret %s: %w", registryKey, err)
	}

	drs, err := utils.ParseDeckhouseRegistrySecret(secret.Data)
	if errors.Is(err, utils.ErrImageRegistryFieldIsNotFound) {
		drs.ImageRegistry = drs.Address + drs.Path
	}

	var discoverySecret corev1.Secret
	discoveryKey := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: app.SecretDiscovery}
	if err := k8sClient.Get(ctx, discoveryKey, &discoverySecret); err != nil {
		return "", "", nil, fmt.Errorf("get discovery secret %s: %w", discoveryKey, err)
	}

	clusterUUID, ok := discoverySecret.Data["clusterUUID"]
	if !ok {
		return "", "", nil, fmt.Errorf("clusterUUID not found in discovery secret %s", discoveryKey)
	}

	releaseChannel := string(discoverySecret.Data["releaseChannel"])

	rconf := &utils.RegistryConfig{
		DockerConfig: drs.DockerConfig,
		Scheme:       drs.Scheme,
		UserAgent:    string(clusterUUID),
		CA:           drs.CA,
	}

	return drs.ImageRegistry, releaseChannel, rconf, nil
}

func getModuleRegistry(ctx context.Context, moduleSource string) (string, *utils.RegistryConfig, error) {
	k8sClient, err := newKubernetesClient()
	if err != nil {
		panic(err)
	}

	ms := new(v1alpha1.ModuleSource)
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleSource}, ms); err != nil {
		return "", nil, fmt.Errorf("get ModuleSource %s got an error: %w", moduleSource, err)
	}

	clusterUUID, _ := getClusterUUID(ctx, k8sClient)
	// TODO: add debug error logging

	rconf := &utils.RegistryConfig{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    clusterUUID,
	}

	return ms.Spec.Registry.Repo, rconf, nil
}

func getClusterUUID(ctx context.Context, client client.Client) (string, error) {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: app.SecretDiscovery}
	err := client.Get(ctx, key, &secret)
	if err != nil {
		return "", fmt.Errorf("read clusterUUID from secret %s failed: %w", key, err)
	}

	clusterUUID, ok := secret.Data["clusterUUID"]
	if !ok {
		return "", fmt.Errorf("key \"clusterUUID\" not defined")
	}

	return string(clusterUUID), nil
}
