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

package layout

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// countingReader counts the scans a reader is asked for: every scan lists the namespaces first.
type countingReader struct {
	client.Reader
	scans int
}

func (r *countingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.NamespaceList); ok {
		r.scans++
	}
	return r.Reader.List(ctx, list, opts...)
}

func platformWithPods(t *testing.T, images ...string) client.Reader {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	objects := []client.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-monitoring"}}}
	for i, image := range images {
		objects = append(objects, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "d8-monitoring", Name: fmt.Sprintf("prometheus-%d", i)},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "prometheus", Image: image}}},
		})
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// TestTakeReusesOnlyWhatCannotAuthorize pins down when a count is asked again. While an air-gap is
// being decided, a count that holds the upstream may be reused for a while, but one that would
// let it go never is; when nothing is being decided any count is reused for longer, since it is
// only reported.
func TestTakeReusesOnlyWhatCannotAuthorize(t *testing.T) {
	ctx := context.Background()
	held := upstream("registry.deckhouse.io")
	start := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	t.Run("a count that holds the upstream, for referenceScanInterval", func(t *testing.T) {
		reader := &countingReader{Reader: platformWithPods(t, upstreamImage)}
		counter := &upstreamReferenceCounter{}

		count, _, err := counter.take(ctx, reader, held, true, start)
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)

		_, _, err = counter.take(ctx, reader, held, true, start.Add(referenceScanInterval/2))
		require.NoError(t, err)
		assert.Equal(t, 1, reader.scans, "reused within the interval")

		_, _, err = counter.take(ctx, reader, held, true, start.Add(referenceScanInterval))
		require.NoError(t, err)
		assert.Equal(t, 2, reader.scans, "and asked again after it")
	})

	t.Run("never a count that would let the upstream go", func(t *testing.T) {
		reader := &countingReader{Reader: platformWithPods(t, inClusterImage)}
		counter := &upstreamReferenceCounter{}

		for i := range 3 {
			count, _, err := counter.take(ctx, reader, held, true, start.Add(time.Duration(i)*time.Second))
			require.NoError(t, err)
			assert.Zero(t, count)
		}
		assert.Equal(t, 3, reader.scans)
	})

	t.Run("any count while nothing is decided, for referenceReportInterval", func(t *testing.T) {
		reader := &countingReader{Reader: platformWithPods(t, inClusterImage)}
		counter := &upstreamReferenceCounter{}

		for _, at := range []time.Duration{0, time.Minute, referenceReportInterval - time.Second} {
			_, _, err := counter.take(ctx, reader, held, false, start.Add(at))
			require.NoError(t, err)
		}
		assert.Equal(t, 1, reader.scans)

		_, _, err := counter.take(ctx, reader, held, false, start.Add(referenceReportInterval))
		require.NoError(t, err)
		assert.Equal(t, 2, reader.scans)
	})

	t.Run("not across a change of upstream", func(t *testing.T) {
		reader := &countingReader{Reader: platformWithPods(t, upstreamImage)}
		counter := &upstreamReferenceCounter{}

		_, _, err := counter.take(ctx, reader, held, false, start)
		require.NoError(t, err)
		count, _, err := counter.take(ctx, reader, upstream("mirror.example.com"), false, start.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, 2, reader.scans)
		assert.Zero(t, count, "counted against the new upstream, not the old one's answer")
	})
}

// TestChangedReportsOnlyAMove keeps the "still names the upstream" line to the passes that say
// something new.
func TestChangedReportsOnlyAMove(t *testing.T) {
	counter := &upstreamReferenceCounter{}

	assert.True(t, counter.changed(3), "the first count is news")
	assert.False(t, counter.changed(3))
	assert.True(t, counter.changed(2))
	assert.False(t, counter.changed(2))
	assert.True(t, counter.changed(0))
}

// pagingReader serves a pod list in the pages a real API server would: at most Limit items, and a
// continue token while more are left. The fake client ignores both, which is why this exists.
type pagingReader struct {
	client.Reader
	pods  []corev1.Pod
	lists int
	limit int64
}

func (r *pagingReader) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	pods, ok := list.(*corev1.PodList)
	if !ok {
		return fmt.Errorf("unexpected list %T", list)
	}
	r.lists++

	options := (&client.ListOptions{}).ApplyOptions(opts)
	r.limit = options.Limit
	offset := 0
	if options.Continue != "" {
		parsed, err := strconv.Atoi(options.Continue)
		if err != nil {
			return err
		}
		offset = parsed
	}
	end := min(offset+int(options.Limit), len(r.pods))

	pods.Items = append(pods.Items[:0], r.pods[offset:end]...)
	pods.Continue = ""
	if end < len(r.pods) {
		pods.Continue = strconv.Itoa(end)
	}
	return nil
}

// TestEachPageReadsEveryPage is what the paging must not lose: every item, read in pages no larger
// than referencePageSize.
func TestEachPageReadsEveryPage(t *testing.T) {
	reader := &pagingReader{}
	for i := range 2*referencePageSize + 1 {
		reader.pods = append(reader.pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i)}})
	}

	seen := map[string]bool{}
	err := eachPage(context.Background(), reader, "d8-monitoring", &corev1.PodList{}, func(list *corev1.PodList) {
		assert.LessOrEqual(t, len(list.Items), referencePageSize)
		for i := range list.Items {
			seen[list.Items[i].Name] = true
		}
	})
	require.NoError(t, err)

	assert.Len(t, seen, 2*referencePageSize+1)
	assert.Equal(t, 3, reader.lists)
	assert.EqualValues(t, referencePageSize, reader.limit)
}
