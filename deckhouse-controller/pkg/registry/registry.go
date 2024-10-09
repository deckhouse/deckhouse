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
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"gopkg.in/alecthomas/kingpin.v2"
)

type DeckhouseRegistrySecret struct {
	DockerConfig          string
	Address               string
	ClusterIsBootstrapped string
	ImageRegistry         string
	Path                  string
	Scheme                string
}

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

		svc := NewService()
		err := svc.InitDeckhouseRegistry(ctx)
		if err != nil {
			return fmt.Errorf("init deckhouse registry service: %w", err)
		}

		tags, err := svc.ListDeckhouseReleases(ctx, *releaseFullList)
		if err != nil {
			return fmt.Errorf("list deckhouse releases: %w", err)
		}

		if len(tags) == 0 {
			fmt.Println()
			fmt.Println("Releases is not found. Try --full argument")

			return nil
		}

		fmt.Println()
		fmt.Println(strings.Join(tags, "\n"))

		return nil
	})

	// deckhouse-controller registry get release <release-channel>
	registryGetReleaseCmd := registryGetCmd.Command("release", "Get release by channel.")
	releaseChannel := registryGetReleaseCmd.Arg("release-channel", "Release channel.").String()
	releaseCompleteInfo := registryGetReleaseCmd.Flag("full", "Complete info.").Bool()
	registryGetReleaseCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		svc := NewService()
		err := svc.InitDeckhouseRegistry(ctx)
		if err != nil {
			return fmt.Errorf("init deckhouse registry service: %w", err)
		}

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

		return nil
	})

	// deckhouse-controller registry list sources
	registryListSourcesCmd := registryListCmd.Command("sources", "List sources")
	registryListSourcesCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.TODO()

		svc := NewService()
		srcs, err := svc.ListModuleSource(ctx)
		if err != nil {
			return fmt.Errorf("list module sources: %w", err)
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

		svc := NewService()
		err := svc.InitModuleRegistry(ctx, *moduleSourceListModules)
		if err != nil {
			return fmt.Errorf("init module registry: %w", err)
		}

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

		svc := NewService()
		err := svc.InitModuleRegistry(ctx, *moduleSourceListModuleRelease)
		if err != nil {
			return fmt.Errorf("init module registry: %w", err)
		}

		tags, err := svc.ListModuleTags(*moduleNameListModuleRelease, *moduleFullList)
		if err != nil {
			return fmt.Errorf("list module tags: %w", err)
		}

		fmt.Println()
		fmt.Println(strings.Join(tags, "\n"))

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

		svc := NewService()
		err := svc.InitModuleRegistry(ctx, *moduleSourceGetModuleRelease)
		if err != nil {
			return fmt.Errorf("init module registry: %w", err)
		}

		meta, err := svc.GetModuleRelease(*moduleNameGetModuleRelease, *moduleChannel)
		if err != nil {
			return fmt.Errorf("get module release %s got an error: %w", *moduleSourceGetModuleRelease, err)
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

func ParseDeckhouseRegistrySecret(data map[string][]byte) (*DeckhouseRegistrySecret, error) {
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

type RegistryInfo struct {
	DockerConfig string
	CA           string
	Scheme       string
	UserAgent    string
}

func GenerateRegistryOptions(ri *RegistryInfo) []cr.Option {
	opts := []cr.Option{
		cr.WithAuth(ri.DockerConfig),
		cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(ri.CA),
		cr.WithInsecureSchema(strings.ToLower(ri.Scheme) == "http"),
	}

	return opts
}
