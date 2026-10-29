/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

// TestHelmReleasesSenderStopsOnCancel guards against a goroutine leak: once one list fails,
// the scan is cancelled and nobody drains releasesC, so the other sender must not block on it.
func TestHelmReleasesSenderStopsOnCancel(t *testing.T) {
	var secret v1.Secret
	if err := yaml.Unmarshal([]byte(helm3ReleaseWithDeprecated), &secret); err != nil {
		t.Fatal(err)
	}
	var cm v1.ConfigMap
	if err := yaml.Unmarshal([]byte(helm2ReleaseWithDeprecated), &cm); err != nil {
		t.Fatal(err)
	}

	resources := []struct {
		name     string
		resource string
		page     runtime.Object
		list     func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error
	}{
		{
			name:     "helm3 secrets",
			resource: "secrets",
			page:     &v1.SecretList{Items: []v1.Secret{secret}},
			list: func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error {
				return h.getHelm3Releases(c, releasesC)
			},
		},
		{
			name:     "helm2 configmaps",
			resource: "configmaps",
			page:     &v1.ConfigMapList{Items: []v1.ConfigMap{cm}},
			list: func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error {
				return h.getHelm2Releases(c, releasesC)
			},
		},
	}

	for _, res := range resources {
		t.Run(res.name, func(t *testing.T) {
			client := fakeHelmClient{Clientset: fake.NewClientset()}
			client.PrependReactor("list", res.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, res.page, nil
			})

			ctx, cancel := context.WithCancelCause(context.Background())
			cancel(errors.New("the other list failed"))
			h := &helmDeprecatedAPIsProcessor{ctx: ctx, cancel: cancel, logger: dlog.NewNop()}

			done := make(chan error, 1)
			go func() { done <- res.list(h, client, make(chan *Release)) }()

			select {
			case err := <-done:
				if err == nil {
					t.Fatal("sender finished without an error on a cancelled scan")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("sender is blocked on releasesC after the scan was cancelled")
			}
		})
	}
}

// TestHelmManifestsCheckFailsOnCancelledScan guards against reporting a partial scan as complete:
// a cancelled scan closes manifestsC too, and select picks between ready cases at random.
func TestHelmManifestsCheckFailsOnCancelledScan(t *testing.T) {
	listErr := errors.New("listing Secrets failed")

	for range 20 {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(listErr)

		manifestsC := make(chan *manifestHead)
		close(manifestsC)

		if _, err := runManifestsCheck(ctx, semver.MustParse("1.33.0"), nil, manifestsC); !errors.Is(err, listErr) {
			t.Fatalf("runManifestsCheck error = %v, want %v", err, listErr)
		}
	}
}
