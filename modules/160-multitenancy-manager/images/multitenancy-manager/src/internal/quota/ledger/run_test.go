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

package ledger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/internal/quota/evaluator"
)

// A leader resumed after a pause longer than the fence still has its leadership context until the
// renew deadline; it must not write the route of the ledger, which another replica may hold by then.
func TestLeaderActsOnlyWithinTheFence(t *testing.T) {
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	lock := &FencedLock{Interface: &slowLock{clock: &clock}, Now: func() time.Time { return clock }}
	r := &runner{timing: DefaultTiming, lock: lock}
	r.synced.Store(true)
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, r.mayAct())

	clock = clock.Add(25 * time.Second)
	assert.False(t, r.mayAct(), "paused past the fence")

	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, r.mayAct())
	r.stopping.Store(true)
	assert.False(t, r.mayAct(), "a stopping leader withdrew its address")
}

// A write of the leader ends with its fence, and a canary report of a leader past its fence is not
// written.
func TestLeaderWritesEndWithTheFence(t *testing.T) {
	clock := time.Now()
	lock := &FencedLock{Interface: &slowLock{clock: &clock}, Now: func() time.Time { return clock }}
	r := &runner{timing: DefaultTiming, lock: lock}
	r.synced.Store(true)
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))

	ctx, cancel := r.withinFence(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.Equal(t, clock.Add(DefaultTiming.Fence), deadline)

	clock = clock.Add(DefaultTiming.Fence)
	assert.ErrorIs(t, r.reportNotCalled(context.Background(), []string{"10.0.0.1:6443"}), errNotActing)
}

// A load balancer whose annotations do not read cleanly is still recorded, so catch-up, which waits
// for an entry of every load balancer it lists, ends: one such Service anywhere in the cluster must
// not keep the ledger from syncing.
func TestOnServiceRecordsEveryLoadBalancer(t *testing.T) {
	r := &runner{state: NewState(), log: logr.Discard()}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "web", ResourceVersion: "7",
			Annotations: map[string]string{"network.deckhouse.io/l2-load-balancer-external-ips-count": "abc"}},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	clusterIP := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "db", ResourceVersion: "8"}}
	r.onService(svc)
	r.onService(clusterIP)
	assert.True(t, r.state.Covers([]metav1.Object{svc, clusterIP}))
	assert.False(t, r.state.KnowsObject(FamilyLB, "other", "db"), "a Service of another type is not recorded")
}

// While the ledger waits for the virtual machines to be served, the webhook of the family is there
// and answers that it cannot count them yet: the webhook configuration has it as soon as the module
// that serves them is enabled, and a missing path would refuse every write with a 404.
func TestRoutesAnswerTheVirtualMachinesNotCountedYet(t *testing.T) {
	r := &runner{opts: Options{Families: []string{FamilyLB}}, state: vmState(), log: logr.Discard(), metrics: NewMetrics(prometheus.NewRegistry()),
		awaitsVirtualMachines: true}
	r.synced.Store(true)
	routes := r.routes()
	response := callObject(t, routes, vmReview(t, admissionv1.Create, "one", 0, 1))
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code)
	assert.Contains(t, response.Result.Message, "not counted yet")
	assert.Zero(t, r.state.ObjectPendingCount())
}

// A quorum list of the virtual machines is covered only once the state holds every machine it lists.
func TestCoversVirtualMachines(t *testing.T) {
	s := vmState()
	listed := newVirtualMachineList()
	for i, name := range []string{"one", "two"} {
		vm := newVirtualMachine()
		vm.SetNamespace("a")
		vm.SetName(name)
		vm.SetResourceVersion(strconv.Itoa(10 + i))
		listed.Items = append(listed.Items, *vm)
	}
	objects := []metav1.Object{&listed.Items[0], &listed.Items[1]}
	s.SetObject(FamilyVM, "a", "one", 10, rl(cores, "1"))
	assert.False(t, s.Covers(objects), "two is not recorded")
	s.SetObject(FamilyVM, "a", "two", 11, rl(cores, "1"))
	assert.True(t, s.Covers(objects))
}

// The approvals that aged are settled by a quorum read of their object: a create that never landed
// gives its place back, and one that landed is taken from the read.
func TestResolveObjectsReadsTheObjects(t *testing.T) {
	landed := newVirtualMachine()
	landed.SetNamespace("a")
	landed.SetName("one")
	landed.SetResourceVersion("12")
	require.NoError(t, unstructured.SetNestedField(landed.Object, int64(3), "spec", "cpu", "cores"))
	require.NoError(t, unstructured.SetNestedField(landed.Object, "100%", "spec", "cpu", "coreFraction"))
	require.NoError(t, unstructured.SetNestedField(landed.Object, "1Gi", "spec", "memory", "size"))
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(virtualMachineGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(virtualMachineGVK.GroupVersion().WithKind(virtualMachineGVK.Kind+"List"), &unstructured.UnstructuredList{})
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(landed).Build()

	state := NewState()
	state.SetProjectQuota(pq("p", rl(cores, "100")))
	state.SetNamespace(ns("a", "p"))
	r := &runner{state: state, log: logr.Discard(), metrics: NewMetrics(prometheus.NewRegistry()), reader: reader}
	require.True(t, r.state.DecideObject(vmWrite("a", "one", 0, "", "3"), testNow, always).Allowed)
	require.True(t, r.state.DecideObject(vmWrite("a", "gone", 0, "", "5"), testNow, always).Allowed)

	r.resolveObjects(context.Background(), testNow, true)
	assert.Zero(t, r.state.ObjectPendingCount())
	assert.False(t, r.state.KnowsObject(FamilyVM, "a", "gone"))
	sum := r.state.projects["p"].familySum[FamilyVM][evaluator.VMCores]
	assert.Equal(t, int64(3), sum.Value())
}

// The webhooks of the families limit the calls of a source on their own: a burst of writes of virtual
// machines through one API server does not refuse the writes of quotas through it, which the quota
// plugin makes with a few workers of its own.
func TestRoutesLimitTheFamiliesApart(t *testing.T) {
	r := &runner{opts: Options{Families: []string{FamilyVM}}, state: vmState(), log: logr.Discard(), metrics: NewMetrics(prometheus.NewRegistry())}
	r.synced.Store(true)
	routes := r.routes()
	vm, _ := routes.Handler(httptest.NewRequest(http.MethodPost, "/validate/quota-vm", nil))
	family, ok := vm.(*ObjectHandler)
	require.True(t, ok)
	ledger, _ := routes.Handler(httptest.NewRequest(http.MethodPost, "/validate/quota-ledger", nil))
	quotas, ok := ledger.(*Handler)
	require.True(t, ok)
	require.NotSame(t, quotas, family.Sources)
	for range perSourceLimit {
		require.True(t, family.Sources.enter("10.0.0.1"))
	}
	assert.True(t, quotas.enter("10.0.0.1"), "the webhook of the quotas has its own places")
}

// A quorum list of the virtual machines as the API server returns it carries the kind of every item, so
// catch-up waits for the state to hold each machine.
func TestCoversAListOfVirtualMachines(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(virtualMachineGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(virtualMachineGVK.GroupVersion().WithKind(virtualMachineGVK.Kind+"List"), &unstructured.UnstructuredList{})
	vm := newVirtualMachine()
	vm.SetNamespace("a")
	vm.SetName("one")
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vm).Build()
	list := newVirtualMachineList()
	require.NoError(t, reader.List(context.Background(), list))
	listed, err := metaObjects(list)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	s := vmState()
	assert.False(t, s.Covers(listed), "the state does not hold the machine yet")
	s.SetObject(FamilyVM, "a", "one", ParseResourceVersion(listed[0].GetResourceVersion()), rl(cores, "1"))
	assert.True(t, s.Covers(listed))
}

// classCache is a cache that lists from a client.
type classCache struct {
	cache.Cache
	reader client.Reader
}

func (c classCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.reader.List(ctx, list, opts...)
}

// Where the MetalLoadBalancerClass kind is served but no class exists, the ledger still holds the
// list of classes, an empty one, from its start: a Service that names a class that does not exist
// counts under the default class, as the metallb module and the controller take it, not under its name.
func TestLoadClassesOfAClusterWithoutClasses(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(metalLoadBalancerClassGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(metalLoadBalancerClassGVK.GroupVersion().WithKind(metalLoadBalancerClassGVK.Kind+"List"), &unstructured.UnstructuredList{})
	require.NoError(t, corev1.AddToScheme(scheme))
	reader := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &runner{state: NewState(), log: logr.Discard(), cache: classCache{reader: reader}}
	r.watchClasses.Store(true)

	r.loadClasses(context.Background())
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "a",
		Annotations: map[string]string{"network.deckhouse.io/metal-load-balancer-class": "foo"}}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}}
	usage, err := liveLoadBalancer{r}.Usage(svc)
	require.NoError(t, err)
	assert.Contains(t, usage, evaluator.LBClassServices("default"))
	assert.NotContains(t, usage, evaluator.LBClassServices("foo"))
}

// followedCache is a cache that lists from a client and records the informers the ledger starts and
// stops.
type followedCache struct {
	classCache
	started, removed int
}

type idleInformer struct{ cache.Informer }

func (idleInformer) AddEventHandler(toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (c *followedCache) GetInformer(context.Context, client.Object, ...cache.InformerGetOption) (cache.Informer, error) {
	c.started++
	return idleInformer{}, nil
}

func (c *followedCache) RemoveInformer(context.Context, client.Object) error {
	c.removed++
	return nil
}

// The kind of the classes may come and go while the ledger runs, with the metallb module: the ledger
// follows it without a restart, and evaluates the load balancers it holds as the kind says.
func TestSetClassesServed(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(metalLoadBalancerClassGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(metalLoadBalancerClassGVK.GroupVersion().WithKind(metalLoadBalancerClassGVK.Kind+"List"), &unstructured.UnstructuredList{})
	require.NoError(t, corev1.AddToScheme(scheme))
	class := newLoadBalancerClass()
	class.SetName("bar")
	require.NoError(t, unstructured.SetNestedField(class.Object, true, "spec", "isDefault"))
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "a", ResourceVersion: "5",
		Annotations: map[string]string{"network.deckhouse.io/metal-load-balancer-class": "foo"}}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(class, svc).Build()
	c := &followedCache{classCache: classCache{reader: reader}}
	r := &runner{state: NewState(), log: logr.Discard(), cache: c}
	ctx := context.Background()
	used := func() corev1.ResourceList { return r.state.objects[FamilyLB][objectKey("a", "web")].used }

	r.onService(svc)
	assert.Contains(t, used(), evaluator.LBClassServices("default"), "without the kind, the annotation of the metallb module is not read")

	require.NoError(t, r.setClassesServed(ctx, true))
	assert.True(t, r.watchClasses.Load())
	assert.Equal(t, 1, c.started)
	assert.Contains(t, used(), evaluator.LBClassServices("bar"), "foo does not exist, so the default class bar counts it")

	require.NoError(t, r.setClassesServed(ctx, true))
	assert.Equal(t, 1, c.started, "a kind served on the next check too is not watched twice")

	require.NoError(t, r.setClassesServed(ctx, false))
	assert.False(t, r.watchClasses.Load())
	assert.Equal(t, 1, c.removed)
	assert.Contains(t, used(), evaluator.LBClassServices("default"), "the kind went away, the annotation is not read again")

	r.onLoadBalancerClass(ctx)
	assert.Contains(t, used(), evaluator.LBClassServices("default"), "a late event of a class does not bring the classes back")

	require.NoError(t, r.setClassesServed(ctx, true))
	assert.Equal(t, 2, c.started)
	assert.Contains(t, used(), evaluator.LBClassServices("bar"), "the kind came back with its classes")
}

// A replica that stepped down itself says so to the other one, which then does not step down to it.
func TestLedgerHealthTellsTheHoldOff(t *testing.T) {
	r := &runner{}
	r.ready.Store(true)
	health := func() map[string]bool {
		rec := httptest.NewRecorder()
		r.healthServer().Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ledger-health", nil))
		state := map[string]bool{}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&state))
		return state
	}
	assert.False(t, health()["holdingOff"])

	r.holdOffTill = time.Now().Add(stepDownHoldOff)
	assert.True(t, health()["holdingOff"])
	assert.True(t, health()["ready"])
}
