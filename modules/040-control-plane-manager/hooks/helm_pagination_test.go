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
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

type fakeHelmClient struct {
	*fake.Clientset
}

func (fakeHelmClient) Dynamic() dynamic.Interface { return nil }

// expiredContinueError is what apiserver returns once etcd compacted the revision a
// continue token points to: 410 with a token resuming at the latest revision.
func expiredContinueError(token string) error {
	err := apierrors.NewResourceExpired("The provided continue parameter is too old")
	err.ErrStatus.ListMeta.Continue = token
	return err
}

// TestHelmReleasesListIsPaginated guards against resourceVersion=0: such a list is
// served from the watch cache, which ignores limit and returns every release at once.
// It also checks that a scan outliving its continue token resumes instead of failing.
func TestHelmReleasesListIsPaginated(t *testing.T) {
	type reply struct {
		next string
		err  error
	}

	resources := []struct {
		name     string
		resource string
		page     func(next string) runtime.Object
		list     func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error
	}{
		{
			name:     "helm3 secrets",
			resource: "secrets",
			page: func(next string) runtime.Object {
				return &v1.SecretList{ListMeta: metav1.ListMeta{Continue: next}}
			},
			list: func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error {
				return h.getHelm3Releases(c, releasesC)
			},
		},
		{
			name:     "helm2 configmaps",
			resource: "configmaps",
			page: func(next string) runtime.Object {
				return &v1.ConfigMapList{ListMeta: metav1.ListMeta{Continue: next}}
			},
			list: func(h *helmDeprecatedAPIsProcessor, c fakeHelmClient, releasesC chan<- *Release) error {
				return h.getHelm2Releases(c, releasesC)
			},
		},
	}

	scenarios := []struct {
		name         string
		replies      []reply
		wantContinue []string // Continue sent on each call
	}{
		{
			name:         "follows continue",
			replies:      []reply{{next: "page-2"}, {}},
			wantContinue: []string{"", "page-2"},
		},
		{
			name:         "resumes an expired continue token",
			replies:      []reply{{next: "page-2"}, {err: expiredContinueError("resumed")}, {}},
			wantContinue: []string{"", "page-2", "resumed"},
		},
	}

	for _, res := range resources {
		for _, sc := range scenarios {
			t.Run(res.name+"/"+sc.name, func(t *testing.T) {
				client := fakeHelmClient{Clientset: fake.NewClientset()}

				var calls []metav1.ListOptions
				client.PrependReactor("list", res.resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
					calls = append(calls, action.(clienttesting.ListActionImpl).GetListOptions())
					if len(calls) > len(sc.replies) {
						t.Fatalf("unexpected list call %d", len(calls))
					}

					r := sc.replies[len(calls)-1]
					if r.err != nil {
						return true, nil, r.err
					}
					return true, res.page(r.next), nil
				})

				h := &helmDeprecatedAPIsProcessor{ctx: context.Background(), logger: dlog.NewNop()}
				if err := res.list(h, client, make(chan *Release, objectBatchSize)); err != nil {
					t.Fatalf("list releases: %v", err)
				}

				if len(calls) != len(sc.wantContinue) {
					t.Fatalf("list calls = %d, want %d", len(calls), len(sc.wantContinue))
				}
				for i, opts := range calls {
					if opts.Limit != objectBatchSize {
						t.Errorf("call %d: limit = %d, want %d", i, opts.Limit, objectBatchSize)
					}
					if opts.ResourceVersion != "" || opts.ResourceVersionMatch != "" {
						t.Errorf("call %d: resourceVersion=%q resourceVersionMatch=%q, want both empty", i, opts.ResourceVersion, opts.ResourceVersionMatch)
					}
					if opts.Continue != sc.wantContinue[i] {
						t.Errorf("call %d: continue = %q, want %q", i, opts.Continue, sc.wantContinue[i])
					}
				}
			})
		}
	}
}
