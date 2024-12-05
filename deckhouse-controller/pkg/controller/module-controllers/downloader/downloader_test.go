/*
Copyright 2023 Flant JSC

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

package downloader

import (
	"os"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestDownloadMetadataFromReleaseChannelError(t *testing.T) {
	ms := &v1alpha1.ModuleSource{}

	dependency.TestDC.CRClient.ImageMock.When("stable").Then(&fake.FakeImage{
		ManifestStub: func() (*v1.Manifest, error) {
			return &v1.Manifest{
				SchemaVersion: 2,
				Layers:        []v1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&utils.FakeLayer{}}, nil
		},
	}, nil)

	md := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, nil)
	_, err := md.DownloadMetadataFromReleaseChannel("commander", "stable", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no version found")
}
