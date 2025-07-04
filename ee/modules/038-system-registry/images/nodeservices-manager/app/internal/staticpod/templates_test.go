/*
Copyright 2025 Flant JSC

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

package staticpod

import (
	"io/fs"
	"testing"
)

func TestTemplatesExists(t *testing.T) {
	count := 0

	err := fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("walk error: %v", err)
		}

		if d.IsDir() {
			return nil
		}

		t.Logf("- %v", path)

		count++

		return nil
	})

	if err != nil {
		t.Fatalf("cannot walk templates directory: %v", err)
	}

	t.Logf("Templates found: %v", count)

	if count == 0 {
		t.Errorf("no templates found")
	}
}

func testRender(t *testing.T, renderer templateRenderer) {
	buf, err := renderer.Render()
	if err != nil {
		t.Fatalf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Fatal("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)
}

func TestStaticPodManifest(t *testing.T) {
	model := staticPodConfigModel{}

	testRender(t, model)
}

func TestStaticPodManifestWithProxy(t *testing.T) {
	model := staticPodConfigModel{
		Proxy: &staticPodProxyModel{},
	}

	testRender(t, model)
}

func TestDistributionConfig(t *testing.T) {
	model := distributionConfigModel{
		Upstream: &distributionConfigUpstreamModel{},
	}

	testRender(t, model)
}

func TestAuthConfigWithMirrorer(t *testing.T) {
	model := authConfigModel{
		RO: authConfigUserModel{
			Name:         "ro-user",
			PasswordHash: "ro-password-hash",
		},
		RW: &authConfigUserModel{
			Name:         "rw-user",
			PasswordHash: "rw-password-hash",
		},
		MirrorPuller: &authConfigUserModel{
			Name:         "puller-user",
			PasswordHash: "puller-password-hash",
		},
		MirrorPusher: &authConfigUserModel{
			Name:         "pusher-user",
			PasswordHash: "pusher-password-hash",
		},
	}

	testRender(t, model)

	model.RW = nil
	testRender(t, model)
}

func TestAuthConfig(t *testing.T) {
	model := authConfigModel{
		RO: authConfigUserModel{
			Name:         "ro-user",
			PasswordHash: "ro-password-hash",
		},
		RW: &authConfigUserModel{
			Name:         "rw-user",
			PasswordHash: "rw-password-hash",
		},
	}
	testRender(t, model)

	model.RW = nil
	testRender(t, model)
}

func TestMirrorerConfig(t *testing.T) {
	model := mirrorerConfigModel{}

	testRender(t, model)
}
