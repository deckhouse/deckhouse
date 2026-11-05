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

package erofs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

var errDownloadInterrupted = fmt.Errorf("%w: %w", cr.ErrImageDownload, context.DeadlineExceeded)

// TestDownloadInterruptedMetric checks that only an interrupted image download fires the alert,
// and that clearing the alert of one module leaves the others firing.
func TestDownloadInterruptedMetric(t *testing.T) {
	storage := newMetricStorage(t)
	i := &Installer{metricStorage: storage}

	console := []map[string]string{{metrics.LabelModule: "console", metrics.LabelVersion: "main"}}

	i.setDownloadInterruptedMetric("console", "main", errDownloadInterrupted)
	i.setDownloadInterruptedMetric("csi-nfs", "main", errors.New("create erofs image: exit status 1"))
	require.Equal(t, console, firingDownloadInterrupted(t, storage))

	i.setDownloadInterruptedMetric("sds-node-configurator", "main", errDownloadInterrupted)
	i.clearDownloadInterruptedMetric("sds-node-configurator")
	require.Equal(t, console, firingDownloadInterrupted(t, storage))

	i.clearDownloadInterruptedMetric("console")
	require.Empty(t, firingDownloadInterrupted(t, storage))
}

// TestDownloadInterruptedMetricClearedOnEarlyExit guards against the alert of an earlier interrupted
// download firing on after the next attempt for the module ends before downloading.
func TestDownloadInterruptedMetricClearedOnEarlyExit(t *testing.T) {
	t.Run("stage finds the image on the filesystem", func(t *testing.T) {
		storage := newMetricStorage(t)
		i := newTestInstaller(t, storage)
		i.setDownloadInterruptedMetric("console", "main", errDownloadInterrupted)

		require.NoError(t, os.MkdirAll(filepath.Join(i.downloaded, "console"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(i.downloaded, "console", "main.erofs"), []byte("image"), 0o644))

		require.NoError(t, i.StageFromRegistry(context.Background(), &v1alpha1.ModuleSource{}, "console", "main"))
		require.Empty(t, firingDownloadInterrupted(t, storage))
	})

	t.Run("restore fails before the download", func(t *testing.T) {
		storage := newMetricStorage(t)
		i := newTestInstaller(t, storage)
		i.setDownloadInterruptedMetric("console", "main", errDownloadInterrupted)

		// the restore stops at closing the old device mapper
		failVeritysetup(t)

		require.Error(t, i.Restore(context.Background(), &v1alpha1.ModuleSource{}, "console", "main"))
		require.Empty(t, firingDownloadInterrupted(t, storage))
	})
}

// TestInterruptedDownloadFiresAlert checks that staging and restoring the module fire the alert
// when the download of the module image breaks off, and that a missing image doesn't fire it.
func TestInterruptedDownloadFiresAlert(t *testing.T) {
	fakeImageTools(t)

	errLayerRead := errors.New("connection reset by peer")
	errNoImage := errors.New("manifest unknown")

	stage := func(i *Installer, source *v1alpha1.ModuleSource) error {
		return i.StageFromRegistry(context.Background(), source, "erofs-test", "main")
	}
	restore := func(i *Installer, source *v1alpha1.ModuleSource) error {
		return i.Restore(context.Background(), source, "erofs-test", "main")
	}

	interrupted := []map[string]string{{metrics.LabelModule: "erofs-test", metrics.LabelVersion: "main"}}

	tests := []struct {
		name     string
		put      func(i *Installer, source *v1alpha1.ModuleSource) error
		image    crv1.Image
		imageErr error
		wantErr  error
		firing   []map[string]string
	}{
		{name: "stage, download breaks off", put: stage, image: brokenOffImage(t, errLayerRead), wantErr: errLayerRead, firing: interrupted},
		{name: "restore, download breaks off", put: restore, image: brokenOffImage(t, errLayerRead), wantErr: errLayerRead, firing: interrupted},
		{name: "stage, no image in the registry", put: stage, imageErr: errNoImage, wantErr: errNoImage},
	}

	source := testSource()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := dependency.NewMockedContainer()
			dc.CRClient.ImageMock.Return(tt.image, tt.imageErr)

			storage := newMetricStorage(t)
			i := newTestInstaller(t, storage)
			i.registry = registry.NewService(dc, log.NewNop())

			err := tt.put(i, source)
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.firing, firingDownloadInterrupted(t, storage))
		})
	}
}

// TestBuildImage checks that only a complete build replaces the image, and that the temporary
// images left by interrupted builds of the version are removed.
func TestBuildImage(t *testing.T) {
	errBuild := errors.New("mkfs.erofs failed")

	t.Run("complete build replaces the image", func(t *testing.T) {
		modulePath := t.TempDir()
		writeFile(t, filepath.Join(modulePath, "main.erofs"), "old image")

		err := buildImage(modulePath, "main", func(path string) error {
			require.Equal(t, modulePath, filepath.Dir(path))
			require.NoFileExists(t, path)

			return os.WriteFile(path, []byte("new image"), 0o644)
		})
		require.NoError(t, err)

		requireFiles(t, modulePath, map[string]string{"main.erofs": "new image"})
	})

	t.Run("failed build keeps the image", func(t *testing.T) {
		modulePath := t.TempDir()
		writeFile(t, filepath.Join(modulePath, "main.erofs"), "old image")

		err := buildImage(modulePath, "main", func(path string) error {
			writeFile(t, path, "partial image")

			return errBuild
		})
		require.ErrorIs(t, err, errBuild)

		requireFiles(t, modulePath, map[string]string{"main.erofs": "old image"})
	})

	t.Run("temporary images of interrupted builds are removed", func(t *testing.T) {
		modulePath := t.TempDir()
		writeFile(t, filepath.Join(modulePath, ".main.erofs.tmp-123"), "partial image")
		writeFile(t, filepath.Join(modulePath, ".v1.0.0.erofs.tmp-456"), "partial image of another version")

		err := buildImage(modulePath, "main", func(path string) error {
			return os.WriteFile(path, []byte("new image"), 0o644)
		})
		require.NoError(t, err)

		requireFiles(t, modulePath, map[string]string{
			"main.erofs":            "new image",
			".v1.0.0.erofs.tmp-456": "partial image of another version",
		})
	})
}

// TestRestoreDownloadsBrokenImageAgain checks that an image on the filesystem that fails to mount
// is downloaded again and replaced instead of failing the restore.
func TestRestoreDownloadsBrokenImageAgain(t *testing.T) {
	fakeImageTools(t)

	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	dc := dependency.NewMockedContainer()
	dc.CRClient.ImageMock.Return(img, nil)

	i := newTestInstaller(t, newMetricStorage(t))
	i.registry = registry.NewService(dc, log.NewNop())

	modulePath := filepath.Join(i.downloaded, "erofs-test")
	require.NoError(t, os.MkdirAll(modulePath, 0o755))
	writeFile(t, filepath.Join(modulePath, "main.erofs"), "broken image")

	require.NoError(t, i.Restore(context.Background(), testSource(), "erofs-test", "main"))

	rc, err := cr.Extract(img)
	require.NoError(t, err)
	tarball, err := io.ReadAll(rc)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(modulePath, "main.erofs"))
	require.NoError(t, err)
	require.Equal(t, tarball, got, "the broken image must be replaced by the downloaded one")
}

// TestRestoreFromRegistry checks that the module image is downloaded even when an image is on the
// filesystem, and that the image on the filesystem is kept and mounted when the download fails.
func TestRestoreFromRegistry(t *testing.T) {
	fakeImageTools(t)

	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	rc, err := cr.Extract(img)
	require.NoError(t, err)
	tarball, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())

	errLayerRead := errors.New("connection reset by peer")
	errNoImage := errors.New("manifest unknown")

	tests := []struct {
		name           string
		onFilesystem   string // the image on the filesystem before the restore, none when empty
		image          crv1.Image
		imageErr       error
		wantDownloaded bool
		wantErr        error
		wantImage      string // the image on the filesystem after the restore, none when empty
		wantMounts     []string
		firing         []map[string]string
	}{
		{
			name:           "complete download replaces the image",
			onFilesystem:   "old image",
			image:          img,
			wantDownloaded: true,
			wantImage:      string(tarball),
			wantMounts:     []string{"erofs-test"},
		},
		{
			name:         "interrupted download mounts the image on the filesystem",
			onFilesystem: "old image",
			image:        brokenOffImage(t, errLayerRead),
			wantImage:    "old image",
			wantMounts:   []string{"erofs-test"},
			firing:       []map[string]string{{metrics.LabelModule: "erofs-test", metrics.LabelVersion: "main"}},
		},
		{
			name:     "failed download without an image fails",
			imageErr: errNoImage,
			wantErr:  errNoImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := dependency.NewMockedContainer()
			dc.CRClient.ImageMock.Return(tt.image, tt.imageErr)

			storage := newMetricStorage(t)
			i := newTestInstaller(t, storage)
			i.registry = registry.NewService(dc, log.NewNop())
			mounts := recordMounts(i)

			modulePath := filepath.Join(i.downloaded, "erofs-test")
			require.NoError(t, os.MkdirAll(modulePath, 0o755))
			if tt.onFilesystem != "" {
				writeFile(t, filepath.Join(modulePath, "main.erofs"), tt.onFilesystem)
			}

			downloaded, err := i.RestoreFromRegistry(context.Background(), testSource(), "erofs-test", "main")
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.wantDownloaded, downloaded)
			require.Equal(t, tt.wantImage, readImage(t, modulePath))
			require.Equal(t, tt.wantMounts, *mounts)
			require.Equal(t, tt.firing, firingDownloadInterrupted(t, storage))
		})
	}
}

// brokenOffImage returns an image whose layer download breaks off with cause after the first bytes.
func brokenOffImage(t *testing.T, cause error) crv1.Image {
	t.Helper()

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	content := bytes.Repeat([]byte("module content\n"), 1024)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "module.yaml", Mode: 0o644, Size: int64(len(content))}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	brokenOff := false
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		if !brokenOff {
			return io.NopCloser(bytes.NewReader(archive.Bytes())), nil
		}

		return io.NopCloser(io.MultiReader(io.LimitReader(bytes.NewReader(archive.Bytes()), 1024), iotest.ErrReader(cause))), nil
	})
	require.NoError(t, err)

	// the layer has read its content to compute the digests, the download comes next
	brokenOff = true

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	return img
}

// fakeImageTools puts on PATH a mkfs.erofs that copies the tar to the image path, the last argument,
// and a veritysetup whose format fails on an image that starts with "broken".
func fakeImageTools(t *testing.T) {
	t.Helper()

	dir := t.TempDir()

	mkfs := "#!/bin/sh\nfor image; do :; done\ncat > \"$image\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mkfs.erofs"), []byte(mkfs), 0o755))

	// format gets the image path before the hash path, the last argument
	veritysetup := "#!/bin/sh\n" +
		"if [ \"$1\" = format ]; then\n" +
		"  for arg; do image=$prev; prev=$arg; done\n" +
		"  [ \"$(head -c 6 \"$image\")\" = broken ] && exit 1\n" +
		"  echo 'Root hash: 0123456789abcdef'\n" +
		"fi\n" +
		"exit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "veritysetup"), []byte(veritysetup), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// readImage returns the content of the main image in the module dir, empty when there is none.
func readImage(t *testing.T, modulePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(modulePath, "main.erofs"))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	require.NoError(t, err)

	return string(content)
}

// requireFiles checks that dir holds exactly the files with the given contents.
func requireFiles(t *testing.T, dir string, want map[string]string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	got := make(map[string]string, len(entries))
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)

		got[entry.Name()] = string(content)
	}

	require.Equal(t, want, got)
}

func newMetricStorage(t *testing.T) *metricsstorage.MetricStorage {
	t.Helper()

	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry(), metricsstorage.WithLogger(log.NewNop()))
	require.NoError(t, metrics.RegisterDeckhouseControllerMetrics(storage))

	return storage
}

func newTestInstaller(t *testing.T, storage metricsstorage.Storage) *Installer {
	t.Helper()

	downloaded := t.TempDir()

	return &Installer{
		downloaded:    downloaded,
		mount:         filepath.Join(downloaded, "modules"),
		metricStorage: storage,
		mountMapper:   func(context.Context, string, string) error { return nil },
		logger:        log.NewNop(),
	}
}

// recordMounts makes the installer record the modules it mounts instead of mounting them.
func recordMounts(i *Installer) *[]string {
	mounts := new([]string)
	i.mountMapper = func(_ context.Context, name, _ string) error {
		*mounts = append(*mounts, name)
		return nil
	}

	return mounts
}

func testSource() *v1alpha1.ModuleSource {
	return &v1alpha1.ModuleSource{
		Spec: v1alpha1.ModuleSourceSpec{
			Registry: v1alpha1.ModuleSourceSpecRegistry{Repo: "registry.example.com/modules"},
		},
	}
}

// failVeritysetup puts a veritysetup that always fails on PATH.
func failVeritysetup(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "veritysetup"), []byte("#!/bin/sh\nexit 1\n"), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// firingDownloadInterrupted returns the label sets the download interrupted gauge is set to 1 for.
func firingDownloadInterrupted(t *testing.T, storage *metricsstorage.MetricStorage) []map[string]string {
	t.Helper()

	families, err := storage.Gather()
	require.NoError(t, err)

	var firing []map[string]string
	for _, family := range families {
		if family.GetName() != metrics.D8ModuleImageDownloadInterrupted {
			continue
		}

		for _, metric := range family.GetMetric() {
			if metric.GetGauge().GetValue() != 1 {
				continue
			}

			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			firing = append(firing, labels)
		}
	}

	return firing
}
