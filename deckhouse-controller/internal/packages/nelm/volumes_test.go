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

package nelm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const volumesNamespace = "app"

// volumeTemplate is a volume claim template of the postgres StatefulSet: its name and size.
type volumeTemplate struct {
	name string
	size string
}

// renderedPostgres is the postgres StatefulSet as the chart renders it.
func renderedPostgres(templates ...volumeTemplate) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{Name: "postgres"},
		Spec: appsv1.StatefulSetSpec{
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "postgres"}},
			ServiceName: "postgres",
		},
	}

	for _, template := range templates {
		sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: template.name},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:   corev1.VolumeResourceRequirements{Requests: storage(template.size)},
			},
		})
	}

	return sts
}

// livePostgres is the postgres StatefulSet as the API server keeps it: with a UID and the defaults
// the server sets.
func livePostgres(templates ...volumeTemplate) *appsv1.StatefulSet {
	sts := renderedPostgres(templates...)
	sts.Namespace = volumesNamespace
	sts.UID = "postgres-uid"
	sts.Spec.Replicas = ptr.To[int32](2)
	sts.Spec.RevisionHistoryLimit = ptr.To[int32](10)
	sts.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement

	for i := range sts.Spec.VolumeClaimTemplates {
		sts.Spec.VolumeClaimTemplates[i].Spec.VolumeMode = ptr.To(corev1.PersistentVolumeFilesystem)
		sts.Spec.VolumeClaimTemplates[i].Status.Phase = corev1.ClaimPending
	}

	return sts
}

// boundClaim is a bound claim labeled as a claim of postgres, in the replicated StorageClass.
func boundClaim(name, size string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: volumesNamespace,
			Labels:    map[string]string{"app": "postgres"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: ptr.To("replicated"),
			Resources:        corev1.VolumeResourceRequirements{Requests: storage(size)},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:    corev1.ClaimBound,
			Capacity: storage(size),
		},
	}
}

// replicated is the StorageClass of the claims; expandable sets whether it allows volume expansion.
func replicated(expandable bool) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta:           metav1.ObjectMeta{Name: "replicated"},
		Provisioner:          "disks.example.com",
		AllowVolumeExpansion: ptr.To(expandable),
	}
}

func storage(size string) corev1.ResourceList {
	return corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
}

// templateSizes is spec.resourceRequests setting the sizes of templates of postgres.
func templateSizes(templates ...volumeTemplate) []resourcesize.Request {
	request := resourcesize.Request{Kind: "StatefulSet", Name: "postgres"}
	for _, template := range templates {
		request.VolumeClaimTemplates = append(request.VolumeClaimTemplates,
			resourcesize.VolumeClaimTemplate{Name: template.name, Storage: template.size})
	}

	return []resourcesize.Request{request}
}

// manifests renders the objects as a multi-document YAML manifest.
func manifests(t *testing.T, objects ...runtime.Object) string {
	t.Helper()

	docs := make([]string, 0, len(objects))
	for _, object := range objects {
		doc, err := yaml.Marshal(object)
		require.NoError(t, err)

		docs = append(docs, string(doc))
	}

	return strings.Join(docs, "---\n")
}

func runExpandClaims(t *testing.T, client *fake.Clientset, requests []resourcesize.Request, rendered ...runtime.Object) error {
	t.Helper()

	return expandStatefulSetClaims(context.Background(), client, log.NewNop(), volumesNamespace, manifests(t, rendered...), requests)
}

// writes lists the changes the step asked of the cluster, as "verb resource/name".
func writes(client *fake.Clientset) []string {
	var changes []string

	for _, action := range client.Actions() {
		switch a := action.(type) {
		case k8stesting.PatchAction:
			changes = append(changes, "patch "+a.GetResource().Resource+"/"+a.GetName())
		case k8stesting.DeleteAction:
			changes = append(changes, "delete "+a.GetResource().Resource+"/"+a.GetName())
		case k8stesting.CreateAction, k8stesting.UpdateAction:
			changes = append(changes, a.GetVerb()+" "+a.GetResource().Resource)
		}
	}

	return changes
}

// sizeOf returns the size the claim in the cluster requests.
func sizeOf(t *testing.T, client *fake.Clientset, name string) string {
	t.Helper()

	claim, err := client.CoreV1().PersistentVolumeClaims(volumesNamespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return claim.Spec.Resources.Requests.Storage().String()
}

func TestExpandClaimsGrowsClaims(t *testing.T) {
	client := fake.NewClientset(
		livePostgres(volumeTemplate{"data", "10Gi"}),
		boundClaim("data-postgres-0", "10Gi"),
		boundClaim("data-postgres-1", "10Gi"),
		// Left behind by a scale down: the StatefulSet reuses it on the next scale up.
		boundClaim("data-postgres-2", "10Gi"),
		replicated(true),
	)

	err := runExpandClaims(t, client, templateSizes(volumeTemplate{"data", "20Gi"}), renderedPostgres(volumeTemplate{"data", "20Gi"}))
	require.NoError(t, err)

	assert.Equal(t, []string{
		"patch persistentvolumeclaims/data-postgres-0",
		"patch persistentvolumeclaims/data-postgres-1",
		"patch persistentvolumeclaims/data-postgres-2",
		"delete statefulsets/postgres",
	}, writes(client))

	for _, name := range []string{"data-postgres-0", "data-postgres-1", "data-postgres-2"} {
		assert.Equal(t, "20Gi", sizeOf(t, client, name), name)
	}

	var options metav1.DeleteOptions
	for _, action := range client.Actions() {
		if del, ok := action.(k8stesting.DeleteAction); ok {
			options = del.GetDeleteOptions()
		}
	}

	assert.Equal(t, ptr.To(metav1.DeletePropagationOrphan), options.PropagationPolicy, "the pods must keep running")
	require.NotNil(t, options.Preconditions)
	assert.Equal(t, ptr.To(types.UID("postgres-uid")), options.Preconditions.UID)
}

func TestExpandClaimsGrowsOnlyClaimsOfTheTemplate(t *testing.T) {
	foreign := boundClaim("data-postgres-1", "10Gi")
	foreign.Labels = map[string]string{"app": "other"}

	client := fake.NewClientset(
		livePostgres(volumeTemplate{"data", "10Gi"}, volumeTemplate{"wal", "1Gi"}),
		boundClaim("data-postgres-0", "10Gi"),
		// Named like a claim of the template, but not made by the StatefulSet.
		foreign,
		// The claim of another template, and of another StatefulSet with the same labels.
		boundClaim("wal-postgres-0", "1Gi"),
		boundClaim("data-postgres-replica-0", "10Gi"),
		replicated(true),
	)

	err := runExpandClaims(t, client,
		templateSizes(volumeTemplate{"data", "20Gi"}),
		renderedPostgres(volumeTemplate{"data", "20Gi"}, volumeTemplate{"wal", "1Gi"}))
	require.NoError(t, err)

	assert.Equal(t, []string{
		"patch persistentvolumeclaims/data-postgres-0",
		"delete statefulsets/postgres",
	}, writes(client))
}

func TestExpandClaimsSkipsUnboundClaims(t *testing.T) {
	pending := boundClaim("data-postgres-1", "10Gi")
	pending.Status = corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending}

	client := fake.NewClientset(
		livePostgres(volumeTemplate{"data", "10Gi"}),
		boundClaim("data-postgres-0", "10Gi"),
		pending,
		replicated(true),
	)

	err := runExpandClaims(t, client, templateSizes(volumeTemplate{"data", "20Gi"}), renderedPostgres(volumeTemplate{"data", "20Gi"}))
	require.NoError(t, err)

	assert.Equal(t, []string{
		"patch persistentvolumeclaims/data-postgres-0",
		"delete statefulsets/postgres",
	}, writes(client))
	assert.Equal(t, "10Gi", sizeOf(t, client, "data-postgres-1"))
}

func TestExpandClaimsGrowsClaimsLeftSmall(t *testing.T) {
	// The template already has the size: the claim was not bound yet when it grew.
	client := fake.NewClientset(
		livePostgres(volumeTemplate{"data", "20Gi"}),
		boundClaim("data-postgres-0", "20Gi"),
		boundClaim("data-postgres-1", "10Gi"),
		replicated(true),
	)

	err := runExpandClaims(t, client, templateSizes(volumeTemplate{"data", "20Gi"}), renderedPostgres(volumeTemplate{"data", "20Gi"}))
	require.NoError(t, err)

	assert.Equal(t, []string{"patch persistentvolumeclaims/data-postgres-1"}, writes(client))
}

func TestExpandClaimsLeavesClusterAlone(t *testing.T) {
	tests := []struct {
		name     string
		live     []runtime.Object
		requests []resourcesize.Request
		rendered []runtime.Object
	}{
		{
			name:     "sizes already applied",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "20Gi"}), boundClaim("data-postgres-0", "20Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
			rendered: []runtime.Object{renderedPostgres(volumeTemplate{"data", "20Gi"})},
		},
		{
			// A first install, or a retry after an install that failed once the StatefulSet was deleted.
			name:     "StatefulSet not in the cluster",
			live:     []runtime.Object{boundClaim("data-postgres-0", "20Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
			rendered: []runtime.Object{renderedPostgres(volumeTemplate{"data", "20Gi"})},
		},
		{
			name:     "templates the requests do not size",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "5Gi"), replicated(true)},
			rendered: []runtime.Object{renderedPostgres(volumeTemplate{"data", "10Gi"})},
		},
		{
			// Not marked for recreation, nelm fails on these changes as it always has.
			name:     "other immutable changes of a StatefulSet the requests do not size",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "10Gi"), replicated(true)},
			rendered: []runtime.Object{withSelector(renderedPostgres(volumeTemplate{"data", "10Gi"}), "postgres-v2")},
		},
		{
			// Nothing is recreated without a resize: nelm fails on the new selector as before.
			name:     "selector changes with the sizes already applied",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "20Gi"}), boundClaim("data-postgres-0", "20Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
			rendered: []runtime.Object{withSelector(renderedPostgres(volumeTemplate{"data", "20Gi"}), "postgres-v2")},
		},
		{
			// nelm would not create a deleted skip-create StatefulSet again.
			name:     "resource policy keeps the StatefulSet",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "10Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
			rendered: []runtime.Object{keptPostgres("keep, skip-create")},
		},
		{
			name:     "StatefulSet the chart does not render",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "10Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
		},
		{
			name:     "template the StatefulSet does not have",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "10Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"wal", "20Gi"}),
			rendered: []runtime.Object{renderedPostgres(volumeTemplate{"data", "10Gi"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(tt.live...)

			require.NoError(t, runExpandClaims(t, client, tt.requests, tt.rendered...))
			assert.Empty(t, writes(client))
		})
	}
}

func TestExpandClaimsReadsOnlyRenderedStatefulSets(t *testing.T) {
	client := fake.NewClientset()

	// A claim the chart renders is applied by nelm, like any other object of the release, and the
	// selector of a Service is not read as a StatefulSet's.
	rendered := "---\n# Source: app/templates/config.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config\n---\n---\n" +
		"apiVersion: v1\nkind: Service\nmetadata:\n  name: postgres\nspec:\n  selector:\n    app: postgres\n---\n" +
		"apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: postgres\nspec:\n  resources:\n    requests:\n      storage: 50Gi\n"

	require.NoError(t, expandStatefulSetClaims(context.Background(), client, log.NewNop(), volumesNamespace, rendered, templateSizes(volumeTemplate{"data", "20Gi"})))
	assert.Empty(t, client.Actions())
}

func TestExpandClaimsSkipsRenderWithoutDisks(t *testing.T) {
	client := fake.NewClientset()

	// Without disk sizes the manifests are not even decoded.
	require.NoError(t, expandStatefulSetClaims(context.Background(), client, log.NewNop(), volumesNamespace, "{not yaml", nil))
	assert.Empty(t, client.Actions())
}

// keptPostgres is the postgres StatefulSet, resized to 20Gi, under the werf resource policy.
func keptPostgres(policy string) *appsv1.StatefulSet {
	sts := renderedPostgres(volumeTemplate{"data", "20Gi"})
	sts.Annotations = map[string]string{"werf.io/resource-policy": policy}

	return sts
}

// withSelector gives the StatefulSet a selector, and pods, of another app label.
func withSelector(sts *appsv1.StatefulSet, app string) *appsv1.StatefulSet {
	sts.Spec.Selector.MatchLabels = map[string]string{"app": app}

	return sts
}

// selectorErr refuses a new selector of postgres along with resized templates.
const selectorErr = "StatefulSet postgres: its selector changes along with the sizes of its volume claim templates, which cannot be applied together"

func TestExpandClaimsRefuses(t *testing.T) {
	tests := []struct {
		name     string
		live     []runtime.Object
		requests []resourcesize.Request
		rendered []runtime.Object
		err      string
	}{
		{
			name:     "template shrinks",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "20Gi"}), boundClaim("data-postgres-0", "20Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "10Gi"}),
			rendered: []runtime.Object{renderedPostgres(volumeTemplate{"data", "10Gi"})},
			err:      "StatefulSet postgres: volume claim template data cannot shrink from 20Gi to 10Gi",
		},
		{
			name:     "selector changes along with a template",
			live:     []runtime.Object{livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "10Gi"), replicated(true)},
			requests: templateSizes(volumeTemplate{"data", "20Gi"}),
			rendered: []runtime.Object{withSelector(renderedPostgres(volumeTemplate{"data", "20Gi"}), "postgres-v2")},
			err:      selectorErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(tt.live...)

			require.EqualError(t, runExpandClaims(t, client, tt.requests, tt.rendered...), tt.err)
			assert.Empty(t, writes(client), "a failed check must leave the cluster as it was")
		})
	}
}

func TestExpandClaimsKeepsStatefulSetWhenClaimRefused(t *testing.T) {
	client := fake.NewClientset(
		livePostgres(volumeTemplate{"data", "10Gi"}),
		boundClaim("data-postgres-0", "10Gi"),
		boundClaim("data-postgres-1", "10Gi"),
		replicated(true),
	)
	client.PrependReactor("patch", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.(k8stesting.PatchAction).GetName() != "data-postgres-1" {
			return false, nil, nil
		}

		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "persistentvolumeclaims"}, "data-postgres-1", errors.New("exceeded quota"))
	})

	err := runExpandClaims(t, client, templateSizes(volumeTemplate{"data", "20Gi"}), renderedPostgres(volumeTemplate{"data", "20Gi"}))
	require.ErrorContains(t, err, "expand PersistentVolumeClaim data-postgres-1 to 20Gi")

	// The StatefulSet stays for a later apply to grow the rest of the claims and recreate it.
	assert.Equal(t, []string{
		"patch persistentvolumeclaims/data-postgres-0",
		"patch persistentvolumeclaims/data-postgres-1",
	}, writes(client))
}

func TestExpandClaimsWaitsUntilTheDeadline(t *testing.T) {
	client := fake.NewClientset(livePostgres(volumeTemplate{"data", "10Gi"}), boundClaim("data-postgres-0", "20Gi"), replicated(true))
	// The garbage collector never gets to remove it.
	client.PrependReactor("delete", "statefulsets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("apply timed out after 20m0s"))

	err := expandStatefulSetClaims(ctx, client, log.NewNop(), volumesNamespace,
		manifests(t, renderedPostgres(volumeTemplate{"data", "20Gi"})), templateSizes(volumeTemplate{"data", "20Gi"}))
	require.EqualError(t, err, "wait for StatefulSet postgres to be deleted: apply timed out after 20m0s")
}

func TestExpandClaimsNamesRefusingStorageClass(t *testing.T) {
	withoutClass := boundClaim("data-postgres-0", "10Gi")
	withoutClass.Spec.StorageClassName = nil

	tests := []struct {
		name string
		live []runtime.Object
		err  string
	}{
		{
			name: "StorageClass without volume expansion",
			live: []runtime.Object{boundClaim("data-postgres-0", "10Gi"), replicated(false)},
			err:  "PersistentVolumeClaim data-postgres-0 cannot be expanded: StorageClass replicated does not allow volume expansion",
		},
		{
			name: "StorageClass not found",
			live: []runtime.Object{boundClaim("data-postgres-0", "10Gi")},
			err:  "PersistentVolumeClaim data-postgres-0 cannot be expanded: StorageClass replicated not found",
		},
		{
			name: "claim without StorageClass",
			live: []runtime.Object{withoutClass},
			err:  "PersistentVolumeClaim data-postgres-0 cannot be expanded: it has no StorageClass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(append(tt.live, livePostgres(volumeTemplate{"data", "10Gi"}))...)
			client.PrependReactor("patch", "persistentvolumeclaims", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "persistentvolumeclaims"}, "data-postgres-0", errors.New("only dynamically provisioned pvc can be resized"))
			})

			err := runExpandClaims(t, client, templateSizes(volumeTemplate{"data", "20Gi"}), renderedPostgres(volumeTemplate{"data", "20Gi"}))
			require.EqualError(t, err, tt.err)
			assert.NotContains(t, writes(client), "delete statefulsets/postgres")
		})
	}
}
