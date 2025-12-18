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

package registryscanner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"registry-modules-watcher/internal/backends"
	"registry-modules-watcher/internal/backends/pkg/registry-scanner/cache"
)

func TestGetMetadataFromImage(t *testing.T) {
	t.Run("parses module.yaml with critical=true", func(t *testing.T) {
		image := createMockImageWithModuleYaml("1.2.3", "name: test-module\ncritical: true\n")

		metadata, err := getMetadataFromImage(image)

		assert.NoError(t, err)
		assert.Equal(t, "1.2.3", metadata.Version)
		assert.True(t, metadata.ModuleDefinitionFound)
		assert.True(t, metadata.ModuleCritical)
	})

	t.Run("parses module.yaml with critical=false", func(t *testing.T) {
		image := createMockImageWithModuleYaml("1.2.3", "name: test-module\ncritical: false\n")

		metadata, err := getMetadataFromImage(image)

		assert.NoError(t, err)
		assert.True(t, metadata.ModuleDefinitionFound)
		assert.False(t, metadata.ModuleCritical)
	})

	t.Run("parses module.yaml without critical field", func(t *testing.T) {
		image := createMockImageWithModuleYaml("1.2.3", "name: test-module\n")

		metadata, err := getMetadataFromImage(image)

		assert.NoError(t, err)
		assert.True(t, metadata.ModuleDefinitionFound)
		assert.False(t, metadata.ModuleCritical) // defaults to false
	})

	t.Run("handles missing module.yaml", func(t *testing.T) {
		image := createMockImageWithModuleYaml("1.2.3", "")

		metadata, err := getMetadataFromImage(image)

		assert.NoError(t, err)
		assert.False(t, metadata.ModuleDefinitionFound)
		assert.False(t, metadata.ModuleCritical)
	})
}

func Test_RegistryScannerProcess(t *testing.T) {
	t.Run("processes initial registry data", func(t *testing.T) {
		mc := minimock.NewController(t)

		clientOne := setupCompleteClientOne(mc)
		clientTwo := setupCompleteClientTwo(mc)

		scanner := &registryscanner{
			logger:          log.NewNop(),
			registryClients: map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo},
			cache:           cache.New(metricsstorage.NewMetricStorage()),
		}

		tasks := scanner.processRegistries(context.Background())

		expectedProcessedTasks := []backends.DocumentationTask{
			{Registry: "clientOne", Module: "console", Version: "1.2.3", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientOne", Module: "console", Version: "2.2.3", ReleaseChannels: []string{"beta"}},
			{Registry: "clientTwo", Module: "console", Version: "3.4.5", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientTwo", Module: "console", Version: "4.4.5", ReleaseChannels: []string{"beta"}},
			{Registry: "clientOne", Module: "parca", Version: "2.3.4", ReleaseChannels: []string{"rock-solid"}},
			{Registry: "clientOne", Module: "parca", Version: "3.3.4", ReleaseChannels: []string{"stable"}},
			{Registry: "clientTwo", Module: "parca", Version: "4.5.6", ReleaseChannels: []string{"rock-solid", "stable"}},
		}

		assertTasksMatch(t, expectedProcessedTasks, tasks)

		expectedCachedTasks := []backends.DocumentationTask{
			{Registry: "clientOne", Module: "console", Version: "1.2.3", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientOne", Module: "console", Version: "2.2.3", ReleaseChannels: []string{"beta"}},
			{Registry: "clientTwo", Module: "console", Version: "3.4.5", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientTwo", Module: "console", Version: "4.4.5", ReleaseChannels: []string{"beta"}},
			{Registry: "clientOne", Module: "parca", Version: "2.3.4", ReleaseChannels: []string{"rock-solid"}},
			{Registry: "clientOne", Module: "parca", Version: "3.3.4", ReleaseChannels: []string{"stable"}},
			{Registry: "clientTwo", Module: "parca", Version: "4.5.6", ReleaseChannels: []string{"rock-solid", "stable"}},
		}

		assertTasksMatch(t, expectedCachedTasks, scanner.cache.GetState())
	})

	t.Run("processes new registry images", func(t *testing.T) {
		mc := minimock.NewController(t)

		clientOne := setupCompleteClientOne(mc)
		clientTwo := setupCompleteClientTwo(mc)

		scanner := &registryscanner{
			logger:          log.NewNop(),
			registryClients: map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo},
			cache:           cache.New(metricsstorage.NewMetricStorage()),
		}

		scanner.processRegistries(context.Background())

		clientOne = setupNewImagesClientOne(mc)
		clientTwo = setupNewImagesClientTwo(mc)

		scanner.registryClients = map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo}

		tasks := scanner.processRegistries(context.Background())

		expectedProcessedTasks := []backends.DocumentationTask{
			{Registry: "clientOne", Module: "console", Version: "2.2.3", ReleaseChannels: []string{"beta"}, Task: backends.TaskDelete},
			{Registry: "clientTwo", Module: "console", Version: "4.4.5", ReleaseChannels: []string{"beta"}, Task: backends.TaskDelete},
			{Registry: "clientOne", Module: "parca", Version: "3.3.4", ReleaseChannels: []string{"stable"}, Task: backends.TaskDelete},
			{Registry: "clientTwo", Module: "parca", Version: "4.5.6", ReleaseChannels: []string{"stable"}, Task: backends.TaskDelete},
			{Registry: "clientOne", Module: "console", Version: "3.3.3", ReleaseChannels: []string{"beta"}},
			{Registry: "clientTwo", Module: "console", Version: "4.4.4", ReleaseChannels: []string{"beta"}},
			{Registry: "clientOne", Module: "parca", Version: "5.5.5", ReleaseChannels: []string{"stable"}},
			{Registry: "clientTwo", Module: "parca", Version: "6.6.6", ReleaseChannels: []string{"stable"}},
		}

		assertTasksMatch(t, expectedProcessedTasks, tasks)

		expectedCachedTasks := []backends.DocumentationTask{
			{Registry: "clientOne", Module: "console", Version: "1.2.3", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientOne", Module: "console", Version: "3.3.3", ReleaseChannels: []string{"beta"}},
			{Registry: "clientTwo", Module: "console", Version: "3.4.5", ReleaseChannels: []string{"alpha"}},
			{Registry: "clientTwo", Module: "console", Version: "4.4.4", ReleaseChannels: []string{"beta"}},
			{Registry: "clientOne", Module: "parca", Version: "2.3.4", ReleaseChannels: []string{"rock-solid"}},
			{Registry: "clientOne", Module: "parca", Version: "5.5.5", ReleaseChannels: []string{"stable"}},
			{Registry: "clientTwo", Module: "parca", Version: "4.5.6", ReleaseChannels: []string{"rock-solid"}},
			{Registry: "clientTwo", Module: "parca", Version: "6.6.6", ReleaseChannels: []string{"stable"}},
		}

		assertTasksMatch(t, expectedCachedTasks, scanner.cache.GetState())
	})
}

func assertTasksMatch(t *testing.T, expected, actual []backends.DocumentationTask) {
	t.Helper()

	assert.Equal(t, len(expected), len(actual), "Cache should have the correct number of entries")

	// Create maps for task lookup using a composite key
	expectedMap := make(map[string]backends.DocumentationTask)
	for _, task := range expected {
		slices.Sort(task.ReleaseChannels)
		key := fmt.Sprintf("%s/%s/%s/%v", task.Registry, task.Module, task.Version, task.ReleaseChannels)
		expectedMap[key] = task
	}

	actualMap := make(map[string]backends.DocumentationTask)
	for _, task := range actual {
		slices.Sort(task.ReleaseChannels)
		key := fmt.Sprintf("%s/%s/%s/%v", task.Registry, task.Module, task.Version, task.ReleaseChannels)
		actualMap[key] = task
	}

	for key, expectedTask := range expectedMap {
		actualTask, exists := actualMap[key]
		assert.True(t, exists, "Expected task not found: %s, task: %d", key, expectedTask.Task)
		if !exists {
			fmt.Println("expected tasks:")
			for k, v := range actualMap {
				fmt.Printf("%s - task: %d\n", k, v.Task)
			}

			continue
		}

		if exists {
			assert.Equal(t, expectedTask.Registry, actualTask.Registry, "Registry mismatch for %s", key)
			assert.Equal(t, expectedTask.Module, actualTask.Module, "Module mismatch for %s", key)
			assert.Equal(t, expectedTask.Version, actualTask.Version, "Version mismatch for %s", key)
			assert.Equal(t, expectedTask.ReleaseChannels, actualTask.ReleaseChannels, "ReleaseChannels mismatch for %s", key)
			assert.Equal(t, 2048, len(actualTask.TarFile), "TarFile length mismatch for %s", key)
			assert.Equal(t, expectedTask.Task, actualTask.Task, "Task mismatch for %s", key)
			assert.Greater(t, len(actualTask.TarFile), 0, "TarFile should not be empty for %s", key)
		}
	}
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
			"5.5.5": createMockImage("c1parcaImageThird", "5.5.5"),
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
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["5.5.5"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "3.3.3").Then(images["console"]["3.3.3"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "5.5.5").Then(images["parca"]["5.5.5"], nil)

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
			"4.4.4": createMockImage("c2consoleImageThird", "4.4.4"),
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
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["4.4.4"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["4.5.6"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["6.6.6"], nil)

	client.ImageMock.When(minimock.AnyContext, "console", "4.4.4").Then(images["console"]["4.4.4"], nil)
	client.ImageMock.When(minimock.AnyContext, "parca", "6.6.6").Then(images["parca"]["6.6.6"], nil)

	return client
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

func createMockImageWithModuleYaml(version, moduleYamlContent string) *crfake.FakeImage {
	filesContent := map[string]string{
		"version.json": `{"version":"` + version + `"}`,
	}
	if moduleYamlContent != "" {
		filesContent["module.yaml"] = moduleYamlContent
	}

	return &crfake.FakeImage{
		DigestStub: func() (v1.Hash, error) {
			return v1.Hash{
				Algorithm: "sha256",
				Hex:       "test",
			}, nil
		},
		ManifestStub: func() (*v1.Manifest, error) {
			return &v1.Manifest{
				Layers: []v1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&FakeLayer{
				FilesContent: filesContent,
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
