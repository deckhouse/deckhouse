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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

var semVerRegex = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?$`)

// Now:
//
// list releases
// list releases --all
// get release alpha
// get release alpha --all
// list sources
// list modules deckhouse-prod
// list module-release deckhouse-prod console
// list module-release deckhouse-prod console --all
// get module-release deckhouse-prod console alpha
// get module-release deckhouse-prod console alpha --all

// Proposal:
//
// get releases
// get releases --all
// get releases -c alpha
// get releases -c alpha --all
// get sources
// get modules deckhouse-prod
// get modules deckhouse-prod -n console
// get modules deckhouse-prod -n console --all
// get modules deckhouse-prod -n console -c alpha
// get modules deckhouse-prod -n console -c alpha --all

func DefineRegistryCommand(kpApp *kingpin.Application) {
	registryCmd := kpApp.Command("registry", "Deckhouse repository work.").
		PreAction(func(_ *kingpin.ParseContext) error {
			kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
			return nil
		})

	registryListCmd := registryCmd.Command("list", "List in registry").
		PreAction(func(_ *kingpin.ParseContext) error {
			kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
			return nil
		})

	registryGetCmd := registryCmd.Command("get", "get from registry").
		PreAction(func(_ *kingpin.ParseContext) error {
			kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
			return nil
		})

	// deckhouse-controller registry list releases
	registryListReleasesCmd := registryListCmd.Command("releases", "List releases.")
	releaseFullList := registryListReleasesCmd.Flag("full", "Complete list of tags.").Bool()
	registryListReleasesCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		registry, rconf, err := getDeckhouseRegistry(ctx)
		if err != nil {
			return fmt.Errorf("get deckhouse registry: %w", err)
		}

		svc := NewDeckhouseService(registry, rconf)

		ls, err := svc.ListDeckhouseReleases()
		if err != nil {
			return fmt.Errorf("list deckhouse releases: %w", err)
		}

		// if we need full tags list, not only semVer
		if !*releaseFullList {
			res := make([]string, 0, 1)
			for _, v := range ls {
				if semVerRegex.MatchString(v) {
					res = append(res, v)
				}
			}

			ls = res
		}

		if len(ls) == 0 {
			fmt.Println()
			fmt.Println("Releases is not found. Try --full argument")

			return nil
		}

		fmt.Println()
		fmt.Println(strings.Join(ls, "\n"))

		return nil
	})

	// deckhouse-controller registry get release <release-channel>
	registryGetReleaseCmd := registryGetCmd.Command("release", "Get release by channel.")
	releaseChannel := registryGetReleaseCmd.Arg("release-channel", "Release channel.").String()
	releaseCompleteInfo := registryGetReleaseCmd.Flag("full", "Complete info.").Bool()
	registryGetReleaseCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		registry, rconf, err := getDeckhouseRegistry(ctx)
		if err != nil {
			return fmt.Errorf("get deckhouse registry: %w", err)
		}

		svc := NewDeckhouseService(registry, rconf)

		meta, err := svc.GetDeckhouseRelease(*releaseChannel)
		if err != nil {
			return fmt.Errorf("get deckhouse release: %w", err)
		}

		if !*releaseCompleteInfo {
			fmt.Println()
			fmt.Printf("Deckhouse version: %s", meta.Version)

			return nil
		}

		b, err := json.MarshalIndent(meta, "", "    ")
		if err != nil {
			return fmt.Errorf("marshall indent: %w", err)
		}

		fmt.Println()
		fmt.Printf("%s", b)
		fmt.Println()

		return nil
	})

	// deckhouse-controller registry list sources
	registryListSourcesCmd := registryListCmd.Command("sources", "List sources")
	registryListSourcesCmd.Action(func(_ *kingpin.ParseContext) error {
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

		fmt.Println()
		fmt.Printf("Module sources found (%d):\n\n", len(srcs))

		for _, src := range srcs {
			fmt.Printf("%s\n", src)
		}

		return nil
	})

	// deckhouse-controller registry list modules <module-source>
	registryListModulesCmd := registryListCmd.Command("modules", "Show modules list.")
	moduleSourceListModules := registryListModulesCmd.Arg("module-source", "Module source name.").String()

	registryListModulesCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		registry, rconf, err := getModuleRegistry(ctx, *moduleSourceListModules)
		if err != nil {
			return fmt.Errorf("get module registry: %w", err)
		}

		svc := NewModuleService(registry, rconf)

		modules, err := svc.ListModules()
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}

		if len(modules) == 0 {
			fmt.Println()
			fmt.Println("Modules is not found. Try --full argument")
			fmt.Println()

			return nil
		}

		fmt.Println()
		fmt.Printf("Module sources found (%d):", len(modules))
		fmt.Println()

		fmt.Println()
		fmt.Println(strings.Join(modules, "\n"))

		return nil
	})

	// deckhouse-controller registry list module-release <module-source> <module-name>
	registryListModuleReleaseCmd := registryListCmd.Command("module-release", "Show modules list.")
	moduleSourceListModuleRelease := registryListModuleReleaseCmd.Arg("module-source", "Module source name.").String()
	moduleNameListModuleRelease := registryListModuleReleaseCmd.Arg("module-name", "Module name.").String()
	moduleFullList := registryListModuleReleaseCmd.Flag("full", "Complete list of tags.").Bool()

	registryListModuleReleaseCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		registry, rconf, err := getModuleRegistry(ctx, *moduleSourceListModuleRelease)
		if err != nil {
			return fmt.Errorf("get module registry: %w", err)
		}

		svc := NewModuleService(registry, rconf)

		ls, err := svc.ListModuleTags(*moduleNameListModuleRelease)
		if err != nil {
			return fmt.Errorf("list module tags: %w", err)
		}

		// if we need full tags list, not only semVer
		if !*moduleFullList {
			res := make([]string, 0, 1)
			for _, v := range ls {
				if semVerRegex.MatchString(v) {
					res = append(res, v)
				}
			}

			ls = res
		}

		if len(ls) == 0 {
			fmt.Println()
			fmt.Println("Module releases is not found.")

			return nil
		}

		fmt.Println()
		fmt.Println(strings.Join(ls, "\n"))

		return nil
	})

	// deckhouse-controller registry get module-release <module-source> <module-name> <module-channel>
	registryGetModuleReleaseCmd := registryGetCmd.Command("module-release", "Show modules list.")
	moduleSourceGetModuleRelease := registryGetModuleReleaseCmd.Arg("module-source", "Module source name.").String()
	moduleNameGetModuleRelease := registryGetModuleReleaseCmd.Arg("module-name", "Module name.").String()
	moduleChannel := registryGetModuleReleaseCmd.Arg("module-channel", "Module name.").String()
	moduleCompleteInfo := registryGetModuleReleaseCmd.Flag("full", "Complete info.").Bool()

	registryGetModuleReleaseCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		registry, rconf, err := getModuleRegistry(ctx, *moduleSourceGetModuleRelease)
		if err != nil {
			return fmt.Errorf("get module registry: %w", err)
		}

		svc := NewModuleService(registry, rconf)

		meta, err := svc.GetModuleRelease(*moduleNameGetModuleRelease, *moduleChannel)
		if err != nil {
			return fmt.Errorf("get module release %s: %w", *moduleNameGetModuleRelease, err)
		}

		if !*moduleCompleteInfo {
			fmt.Println()
			fmt.Printf("Module version: %s", meta.Version)
			fmt.Println()

			return nil
		}

		b, err := json.MarshalIndent(meta, "", "    ")
		if err != nil {
			return fmt.Errorf("marshall indent: %w", err)
		}

		fmt.Println()
		fmt.Printf("%s", b)
		fmt.Println()

		return nil
	})
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

func getDeckhouseRegistry(ctx context.Context) (string, *RegistryConfig, error) {
	k8sClient, err := newKubernetesClient()
	if err != nil {
		panic(err)
	}

	secret := new(corev1.Secret)
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-registry"}, secret); err != nil {
		return "", nil, fmt.Errorf("list ModuleSource got an error: %w", err)
	}

	drs, err := parseDeckhouseRegistrySecret(secret.Data)
	if err != nil {
		return "", nil, fmt.Errorf("parse deckhouse registry secret: %w", err)
	}

	var discoverySecret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	if err := k8sClient.Get(ctx, key, &discoverySecret); err != nil {
		return "", nil, fmt.Errorf("get deckhouse discovery sectret got an error: %w", err)
	}

	clusterUUID, ok := discoverySecret.Data["clusterUUID"]
	if !ok {
		return "", nil, fmt.Errorf("not found clusterUUID in discovery secret: %w", err)
	}

	rconf := &RegistryConfig{
		DockerConfig: drs.DockerConfig,
		Scheme:       drs.Scheme,
		UserAgent:    string(clusterUUID),
	}

	return drs.ImageRegistry, rconf, nil
}

func getModuleRegistry(ctx context.Context, moduleSource string) (string, *RegistryConfig, error) {
	k8sClient, err := newKubernetesClient()
	if err != nil {
		panic(err)
	}

	ms := new(v1alpha1.ModuleSource)
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleSource}, ms); err != nil {
		return "", nil, fmt.Errorf("get ModuleSource %s got an error: %w", moduleSource, err)
	}

	rconf := &RegistryConfig{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    "deckhouse-controller/ModuleControllers",
	}

	return ms.Spec.Registry.Repo, rconf, nil
}

type DeckhouseRegistrySecret struct {
	DockerConfig          string
	Address               string
	ClusterIsBootstrapped string
	ImageRegistry         string
	Path                  string
	Scheme                string
}

func parseDeckhouseRegistrySecret(data map[string][]byte) (*DeckhouseRegistrySecret, error) {
	dockerConfig, ok := data[".dockerconfigjson"]
	if !ok {
		return nil, errors.New("secret has no .dockerconfigjson field")
	}

	address, ok := data["address"]
	if !ok {
		return nil, errors.New("secret has no address field")
	}

	clusterIsBootstrapped, ok := data["clusterIsBootstrapped"]
	if !ok {
		return nil, errors.New("secret has no clusterIsBootstrapped field")
	}

	imagesRegistry, ok := data["imagesRegistry"]
	if !ok {
		return nil, errors.New("secret has no imagesRegistry field")
	}

	path, ok := data["path"]
	if !ok {
		return nil, errors.New("secret has no path field")
	}

	scheme, ok := data["scheme"]
	if !ok {
		return nil, errors.New("secret has no scheme field")
	}

	return &DeckhouseRegistrySecret{
		DockerConfig:          string(dockerConfig),
		Address:               string(address),
		ClusterIsBootstrapped: string(clusterIsBootstrapped),
		ImageRegistry:         string(imagesRegistry),
		Path:                  string(path),
		Scheme:                string(scheme),
	}, nil
}

type RegistryConfig struct {
	DockerConfig string
	CA           string
	Scheme       string
	UserAgent    string
}

func GenerateRegistryOptions(ri *RegistryConfig) []cr.Option {
	opts := []cr.Option{
		cr.WithAuth(ri.DockerConfig),
		cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(ri.CA),
		cr.WithInsecureSchema(strings.ToLower(ri.Scheme) == "http"),
	}

	return opts
}
