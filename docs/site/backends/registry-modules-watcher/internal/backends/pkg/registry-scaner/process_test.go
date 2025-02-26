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

package registryscaner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"registry-modules-watcher/internal/backends/pkg/registry-scaner/cache"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/gojuno/minimock/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"
)

func Test_RegistryScannerProcess(t *testing.T) {
	mc := minimock.NewController(t)

	clientOne := setupCompleteClientOne(mc)
	clientTwo := setupCompleteClientTwo(mc)

	// Create scanner with mocked dependencies
	scanner := &registryscaner{
		logger:          log.NewNop(),
		registryClients: map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo},
		cache:           cache.New(),
	}

	// Run the scanner
	scanner.processRegistries(context.Background())

	expectedCache := buildCompleteExpectedCache()
	assert.Equal(t, expectedCache, scanner.cache.GetCache())

	fmt.Println("second test")
	clientOne = setupNewImagesClientOne(mc)
	clientTwo = setupNewImagesClientTwo(mc)

	scanner.registryClients = map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo}

	// Run the scanner
	scanner.processRegistries(context.Background())

	expectedCache = buildUpdatedExpectedCache()
	assert.Equal(t, expectedCache, scanner.cache.GetCache())
}

func setupCompleteClientOne(mc *minimock.Controller) Client {
	images := map[string]map[string]*crfake.FakeImage{
		"console": {
			"1.2.3": createMockImage("c1consoleImageFirst", "1.2.3"),
			"2.2.3": createMockImage("c1consoleImageSecond", "2.2.3"),
		},
		"parca": {
			"2.3.4": createMockImage("c1parcaImageFirst", "2.3.4"),
			"3.3.4": createMockImage("c1parcaImageSecond", "3.3.4"),
		},
	}

	client := NewClientMock(mc)
	client.NameMock.Return("clientOne")
	client.ModulesMock.Return([]string{"console", "parca"}, nil)

	client.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha", "beta"}, nil)
	client.ListTagsMock.When(minimock.AnyContext, "parca").Then([]string{"rock-solid", "stable"}, nil)

	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["1.2.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["2.2.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["2.3.4"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["3.3.4"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "1.2.3").Then(images["console"]["1.2.3"], nil)
	client.ImageMock.When(minimock.AnyContext, "console", "2.2.3").Then(images["console"]["2.2.3"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "2.3.4").Then(images["parca"]["2.3.4"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "3.3.4").Then(images["parca"]["3.3.4"], nil)

	return client
}

func setupNewImagesClientOne(mc *minimock.Controller) Client {
	images := map[string]map[string]*crfake.FakeImage{
		"console": {
			"1.2.3": createMockImage("c1consoleImageFirst", "1.2.3"),
			"3.3.3": createMockImage("c1consoleImageThird", "3.3.3"),
		},
		"parca": {
			"2.3.4": createMockImage("c1parcaImageFirst", "2.3.4"),
			"4.4.4": createMockImage("c1parcaImageThird", "4.4.4"),
		},
	}

	client := NewClientMock(mc)
	client.NameMock.Return("clientOne")
	client.ModulesMock.Return([]string{"console", "parca"}, nil)

	client.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha", "beta"}, nil)
	client.ListTagsMock.When(minimock.AnyContext, "parca").Then([]string{"rock-solid", "stable"}, nil)

	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["1.2.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["3.3.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["2.3.4"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["4.4.4"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "3.3.3").Then(images["console"]["3.3.3"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "4.4.4").Then(images["parca"]["4.4.4"], nil)

	return client
}

func setupCompleteClientTwo(mc *minimock.Controller) Client {
	images := map[string]map[string]*crfake.FakeImage{
		"console": {
			"3.4.5": createMockImage("c2consoleImageFirst", "3.4.5"),
			"4.4.5": createMockImage("c2consoleImageSecond", "4.4.5"),
		},
		"parca": {
			"4.5.6": createMockImage("c2parcaImageFirst", "4.5.6"),
		},
	}

	client := NewClientMock(mc)
	client.NameMock.Return("clientTwo")
	client.ModulesMock.Return([]string{"console", "parca"}, nil)

	client.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha", "beta"}, nil)
	client.ListTagsMock.When(minimock.AnyContext, "parca").Then([]string{"rock-solid", "stable"}, nil)

	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["3.4.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["4.4.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["4.5.6"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["4.5.6"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "3.4.5").Then(images["console"]["3.4.5"], nil)
	client.ImageMock.When(minimock.AnyContext, "console", "4.4.5").Then(images["console"]["4.4.5"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "4.5.6").Then(images["parca"]["4.5.6"], nil)

	return client
}

func setupNewImagesClientTwo(mc *minimock.Controller) Client {
	images := map[string]map[string]*crfake.FakeImage{
		"console": {
			"3.4.5": createMockImage("c2consoleImageFirst", "3.4.5"),
			"5.5.5": createMockImage("c2consoleImageThird", "5.5.5"),
		},
		"parca": {
			"4.5.6": createMockImage("c2parcaImageFirst", "4.5.6"),
			"6.6.6": createMockImage("c2parcaImageThird", "6.6.6"),
		},
	}

	client := NewClientMock(mc)
	client.NameMock.Return("clientTwo")
	client.ModulesMock.Return([]string{"console", "parca"}, nil)

	client.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha", "beta"}, nil)
	client.ListTagsMock.When(minimock.AnyContext, "parca").Then([]string{"rock-solid", "stable"}, nil)

	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["3.4.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["5.5.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["4.5.6"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["6.6.6"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "5.5.5").Then(images["console"]["5.5.5"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "6.6.6").Then(images["parca"]["6.6.6"], nil)

	return client
}

func buildCompleteExpectedCache() map[cache.RegistryName]map[cache.ModuleName]cache.ModuleData {
	return map[cache.RegistryName]map[cache.ModuleName]cache.ModuleData{
		"clientOne": {
			"console": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"alpha": "algo:c1consoleImageFirst",
					"beta":  "algo:c1consoleImageSecond",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"1.2.3": {
						ReleaseChannels: map[string]struct{}{"alpha": {}},
						TarLen:          1536,
					},
					"2.2.3": {
						ReleaseChannels: map[string]struct{}{"beta": {}},
						TarLen:          1536,
					},
				},
			},
			"parca": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"rock-solid": "algo:c1parcaImageFirst",
					"stable":     "algo:c1parcaImageSecond",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"2.3.4": {
						ReleaseChannels: map[string]struct{}{"rock-solid": {}},
						TarLen:          1536,
					},
					"3.3.4": {
						ReleaseChannels: map[string]struct{}{"stable": {}},
						TarLen:          1536,
					},
				},
			},
		},
		"clientTwo": {
			"console": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"alpha": "algo:c2consoleImageFirst",
					"beta":  "algo:c2consoleImageSecond",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"3.4.5": {
						ReleaseChannels: map[string]struct{}{"alpha": {}},
						TarLen:          1536,
					},
					"4.4.5": {
						ReleaseChannels: map[string]struct{}{"beta": {}},
						TarLen:          1536,
					},
				},
			},
			"parca": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"rock-solid": "algo:c2parcaImageFirst",
					"stable":     "algo:c2parcaImageFirst",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"4.5.6": {
						ReleaseChannels: map[string]struct{}{
							"rock-solid": {},
							"stable":     {},
						},
						TarLen: 1536,
					},
				},
			},
		},
	}
}

func createMockImage(hex, version string) *crfake.FakeImage {
	return &crfake.FakeImage{
		DigestStub: func() (v1.Hash, error) {
			return v1.Hash{
				Algorithm: "algo",
				Hex:       hex,
			}, nil
		},
		ManifestStub: func() (*v1.Manifest, error) {
			return &v1.Manifest{
				Layers: []v1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&FakeLayer{
				FilesContent: map[string]string{
					"version.json": `{"version":"` + version + `"}`,
				},
			}}, nil
		},
	}
}

type FakeLayer struct {
	v1.Layer

	FilesContent map[string]string // pair: filename - file content
}

func (fl FakeLayer) Uncompressed() (io.ReadCloser, error) {
	result := bytes.NewBuffer(nil)
	if fl.FilesContent == nil {
		fl.FilesContent = make(map[string]string)
	}

	if len(fl.FilesContent) == 0 {
		return io.NopCloser(result), nil
	}

	wr := tar.NewWriter(result)

	// create files in a single layer
	for filename, content := range fl.FilesContent {
		if strings.Contains(filename, "/") {
			dirs := strings.Split(filename, "/")
			for i := 0; i < len(dirs)-1; i++ {
				hdr := &tar.Header{
					Name:     dirs[i],
					Typeflag: tar.TypeDir,
					Mode:     0777,
				}
				_ = wr.WriteHeader(hdr)
			}
		}

		hdr := &tar.Header{
			Name:     filename,
			Typeflag: tar.TypeReg,
			Mode:     0600,
			Size:     int64(len(content)),
		}
		_ = wr.WriteHeader(hdr)
		_, _ = wr.Write([]byte(content))
	}
	_ = wr.Close()

	return io.NopCloser(result), nil
}

func (fl FakeLayer) Size() (int64, error) {
	return int64(len(fl.FilesContent)), nil
}

func buildUpdatedExpectedCache() map[cache.RegistryName]map[cache.ModuleName]cache.ModuleData {
	return map[cache.RegistryName]map[cache.ModuleName]cache.ModuleData{
		"clientOne": {
			"console": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"alpha": "algo:c1consoleImageFirst",
					"beta":  "algo:c1consoleImageThird",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"1.2.3": {
						ReleaseChannels: map[string]struct{}{"alpha": {}},
						TarLen:          1536,
					},
					"3.3.3": {
						ReleaseChannels: map[string]struct{}{"beta": {}},
						TarLen:          1536,
					},
				},
			},
			"parca": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"rock-solid": "algo:c1parcaImageFirst",
					"stable":     "algo:c1parcaImageThird",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"2.3.4": {
						ReleaseChannels: map[string]struct{}{"rock-solid": {}},
						TarLen:          1536,
					},
					"4.4.4": {
						ReleaseChannels: map[string]struct{}{"stable": {}},
						TarLen:          1536,
					},
				},
			},
		},
		"clientTwo": {
			"console": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"alpha": "algo:c2consoleImageFirst",
					"beta":  "algo:c2consoleImageThird",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"3.4.5": {
						ReleaseChannels: map[string]struct{}{"alpha": {}},
						TarLen:          1536,
					},
					"5.5.5": {
						ReleaseChannels: map[string]struct{}{"beta": {}},
						TarLen:          1536,
					},
				},
			},
			"parca": {
				ReleaseChecksum: map[cache.ReleaseChannelName]string{
					"rock-solid": "algo:c2parcaImageFirst",
					"stable":     "algo:c2parcaImageThird",
				},
				Versions: map[cache.VersionNum]cache.Data{
					"4.5.6": {
						ReleaseChannels: map[string]struct{}{"rock-solid": {}},
						TarLen:          1536,
					},
					"6.6.6": {
						ReleaseChannels: map[string]struct{}{"stable": {}},
						TarLen:          1536,
					},
				},
			},
		},
	}
}
