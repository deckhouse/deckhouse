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

func Test_RegistryScannerProcess(t *testing.T) {
	t.Run("processes initial registry data", func(t *testing.T) {
		mc := minimock.NewController(t)

		clientOne := setupCompleteClientOne(mc)
		clientTwo := setupCompleteClientTwo(mc)

		scanner := &registryscanner{
			logger:          log.NewNop(),
			registryClients: map[string]Client{"clientOne": clientOne, "clientTwo": clientTwo},
			cache:           cache.New(metricsstorage.NewMetricStorage("test")),
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
			cache:           cache.New(metricsstorage.NewMetricStorage("test")),
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

	t.Run("verifies cache hit optimization - only ReleaseDigest called", func(t *testing.T) {
		mc := minimock.NewController(t)

		// Setup initial client with data - only console for simplicity
		clientOne := NewClientMock(mc)
		clientOne.NameMock.Return("clientOne")
		clientOne.ModulesMock.Return([]string{"console"}, nil)
		clientOne.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha"}, nil)

		// First run expectations - cache miss, needs both ReleaseDigest and ReleaseImage
		clientOne.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c1consoleImageFirst", nil)
		clientOne.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(createMockImage("c1consoleImageFirst", "1.2.3"), nil)

		scanner := &registryscanner{
			logger:          log.NewNop(),
			registryClients: map[string]Client{"clientOne": clientOne},
			cache:           cache.New(metricsstorage.NewMetricStorage("test")),
		}

		// First run - populates cache
		scanner.processRegistries(context.Background())

		// Consume the initial tasks state (simulating that they were processed)
		scanner.cache.GetState()

		// Setup client for second run - same digests (cache hits)
		clientOneCacheHit := NewClientMock(mc)
		clientOneCacheHit.NameMock.Return("clientOne")
		clientOneCacheHit.ModulesMock.Return([]string{"console"}, nil)
		clientOneCacheHit.ListTagsMock.When(minimock.AnyContext, "console").Then([]string{"alpha"}, nil)

		// Only ReleaseDigest should be called (cache hit)
		clientOneCacheHit.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c1consoleImageFirst", nil)

		// ReleaseImage and Image should NOT be called (cache hit optimization)
		// No expectations set = test will fail if these methods are called

		scanner.registryClients = map[string]Client{"clientOne": clientOneCacheHit}

		// Second run - should use cache (no new tasks)
		tasks := scanner.processRegistries(context.Background())

		// Should be empty since nothing changed
		assert.Empty(t, tasks, "No tasks should be generated for cache hits")

		// Cache should still have the original data
		cachedTasks := scanner.cache.GetState()
		foundTask := false
		for _, task := range cachedTasks {
			if task.Registry == "clientOne" && task.Module == "console" && task.Version == "1.2.3" {
				foundTask = true
				assert.Equal(t, []string{"alpha"}, task.ReleaseChannels)
				assert.Greater(t, len(task.TarFile), 0, "TarFile should not be empty")
				break
			}
		}
		assert.True(t, foundTask, "Expected cached task should be found")
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

	// ReleaseDigest is always called first for cache check (lightweight operation)
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c1consoleImageFirst", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "beta").Then("c1consoleImageSecond", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "rock-solid").Then("c1parcaImageFirst", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "stable").Then("c1parcaImageSecond", nil)

	// ReleaseImage is called on cache miss (new initial data scenario)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["1.2.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["2.2.3"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["2.3.4"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["3.3.4"], nil)

	// Image calls are now rare (fallback only) - removed as optimized code uses already loaded Image from ReleaseImage

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

	// ReleaseDigest is always called first for cache check
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c1consoleImageFirst", nil)      // Cache hit - same digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "beta").Then("c1consoleImageThird", nil)       // Cache miss - new digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "rock-solid").Then("c1parcaImageFirst", nil)     // Cache hit - same digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "stable").Then("c1parcaImageThird", nil)         // Cache miss - new digest

	// ReleaseImage is only called for cache misses (changed digests)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["3.3.3"], nil)   // Cache miss
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["5.5.5"], nil)     // Cache miss

	// Image calls are now rare (fallback only) - removed as optimized code uses already loaded Image from ReleaseImage

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

	// ReleaseDigest is always called first for cache check (lightweight operation)
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c2consoleImageFirst", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "beta").Then("c2consoleImageSecond", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "rock-solid").Then("c2parcaImageFirst", nil)
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "stable").Then("c2parcaImageFirst", nil)  // Same image for both channels

	// ReleaseImage is called on cache miss (new initial data scenario)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "alpha").Then(images["console"]["3.4.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["4.4.5"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "rock-solid").Then(images["parca"]["4.5.6"], nil)
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["4.5.6"], nil)

	// Image calls are now rare (fallback only) - removed as optimized code uses already loaded Image from ReleaseImage

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

	// ReleaseDigest is always called first for cache check
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "alpha").Then("c2consoleImageFirst", nil)      // Cache hit - same digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "console", "beta").Then("c2consoleImageThird", nil)       // Cache miss - new digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "rock-solid").Then("c2parcaImageFirst", nil)     // Cache hit - same digest
	client.ReleaseDigestMock.When(minimock.AnyContext, "parca", "stable").Then("c2parcaImageThird", nil)         // Cache miss - new digest

	// ReleaseImage is only called for cache misses (changed digests)
	client.ReleaseImageMock.When(minimock.AnyContext, "console", "beta").Then(images["console"]["4.4.4"], nil)   // Cache miss
	client.ReleaseImageMock.When(minimock.AnyContext, "parca", "stable").Then(images["parca"]["6.6.6"], nil)     // Cache miss

	// Image calls are now rare (fallback only) - removed as optimized code uses already loaded Image from ReleaseImage

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
