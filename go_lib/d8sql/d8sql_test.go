package d8sql

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deckhouse/d8sql/sql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

var (
	podGVR  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	nodeGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	cmGVR   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	depGVR  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
)

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper(nil)
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Node"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
	return m
}

func listKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		podGVR:  "PodList",
		nodeGVR: "NodeList",
		cmGVR:   "ConfigMapList",
		depGVR:  "DeploymentList",
	}
}

func pod(ns, name, phase, node string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"namespace": ns, "name": name},
		"spec":       map[string]any{"nodeName": node},
		"status":     map[string]any{"phase": phase},
	}}
}

func podLabeled(ns, name string, lbls map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"namespace": ns, "name": name, "labels": lbls},
		"status":     map[string]any{"phase": "Running"},
	}}
}

func node(name, instanceType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata": map[string]any{
			"name":   name,
			"labels": map[string]any{"node.kubernetes.io/instance-type": instanceType},
		},
	}}
}

func configmap(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"namespace": ns, "name": name},
		"data":       map[string]any{"foo": "old"},
	}}
}

func deployment(ns, name string, replicas, ready int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"namespace": ns, "name": name},
		"spec":       map[string]any{"replicas": replicas},
		"status":     map[string]any{"readyReplicas": ready},
	}}
}

func newEngine(objs ...runtime.Object) (*Engine, *dynfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	c := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds(), objs...)
	return New(c, testMapper()), c
}

func TestSelectRunningPods(t *testing.T) {
	e, _ := newEngine(
		pod("default", "redis-0", "Running", "n1"),
		pod("default", "redis-1", "Pending", "n1"),
		pod("default", "web", "Running", "n2"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name, status.phase FROM pods WHERE status.phase = 'Running'")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %d (%v)", len(res.Rows), res.Rows)
	}
	for _, r := range res.Rows {
		if r[1] != "Running" {
			t.Errorf("phase: %v", r[1])
		}
	}
}

func TestSelectNamespacePushdown(t *testing.T) {
	e, _ := newEngine(
		pod("default", "a", "Running", "n1"),
		pod("d8-system", "b", "Running", "n1"),
		pod("d8-system", "c", "Running", "n1"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.namespace = 'd8-system'")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %d", len(res.Rows))
	}
}

func TestSelectAllNamespacesByDefault(t *testing.T) {
	// With no metadata.namespace constraint, queries span all namespaces.
	e, _ := newEngine(
		pod("default", "a", "Running", "n1"),
		pod("other", "b", "Running", "n1"),
	)
	res, err := e.ExecuteOne(context.Background(), "SELECT metadata.name FROM pods")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected pods from all namespaces, got rows: %v", res.Rows)
	}
}

func TestSelectScopedNamespace(t *testing.T) {
	// WithDefaultNamespace scopes queries that omit metadata.namespace.
	e, _ := newEngine(
		pod("default", "a", "Running", "n1"),
		pod("other", "b", "Running", "n1"),
	)
	e.defaultNS = "default"
	res, err := e.ExecuteOne(context.Background(), "SELECT metadata.name FROM pods")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "a" {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestSelectLabelPushdown(t *testing.T) {
	e, c := newEngine(
		podLabeled("default", "a", map[string]any{"app": "redis"}),
		podLabeled("default", "b", map[string]any{"app": "memcached"}),
	)
	var gotSel string
	c.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if la, ok := action.(clienttesting.ListAction); ok {
			gotSel = la.GetListRestrictions().Labels.String()
		}
		return false, nil, nil // fall through to the default tracker
	})
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.labels.'app' = 'redis'")
	if err != nil {
		t.Fatal(err)
	}
	if gotSel != "app=redis" {
		t.Errorf("expected label selector pushed to server, got %q", gotSel)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "a" {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestSelectLabelPushdownSetBased(t *testing.T) {
	e, c := newEngine(
		podLabeled("default", "a", map[string]any{"app": "redis"}),
		podLabeled("default", "b", map[string]any{"app": "memcached"}),
		podLabeled("default", "c", map[string]any{"tier": "cache"}),
	)
	var gotSel string
	c.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if la, ok := action.(clienttesting.ListAction); ok {
			gotSel = la.GetListRestrictions().Labels.String()
		}
		return false, nil, nil
	})
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.labels.'app' IN ('redis', 'memcached')")
	if err != nil {
		t.Fatal(err)
	}
	if gotSel != "app in (memcached,redis)" {
		t.Errorf("expected set-based label selector, got %q", gotSel)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestPageSizeStreaming(t *testing.T) {
	// With a small page size the pager streams items; results must be identical.
	objs := []runtime.Object{
		configmap("default", "a"),
		configmap("default", "b"),
		configmap("kube-system", "c"),
		configmap("kube-system", "d"),
		configmap("other", "e"),
	}
	e, c := newEngine(objs...)
	e.pageSize = 2

	res, err := e.ExecuteOne(context.Background(), "SELECT * FROM configmaps")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Objects) != 5 {
		t.Fatalf("expected 5 streamed configmaps, got %d", len(res.Objects))
	}

	del, err := e.ExecuteOne(context.Background(), "DELETE FROM configmaps")
	if err != nil {
		t.Fatal(err)
	}
	if del.Affected != 5 {
		t.Fatalf("expected 5 deleted, got %d", del.Affected)
	}
	left, err := c.Resource(cmGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(left.Items) != 0 {
		t.Fatalf("expected all configmaps deleted, %d left", len(left.Items))
	}
}

func TestSelectLike(t *testing.T) {
	e, _ := newEngine(
		pod("default", "my-redis-master", "Running", "n1"),
		pod("default", "postgres-0", "Running", "n1"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.name LIKE '%redis%'")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "my-redis-master" {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestSelectStarClusterScoped(t *testing.T) {
	e, _ := newEngine(node("n1", "m5.large"), node("n2", "m5.xlarge"))
	res, err := e.ExecuteOne(context.Background(), "SELECT * FROM nodes")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Objects) != 2 {
		t.Fatalf("objects: %d", len(res.Objects))
	}
}

func TestSelectFieldToField(t *testing.T) {
	e, _ := newEngine(
		deployment("default", "ready", 3, 3),
		deployment("default", "degraded", 3, 1),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM deployments WHERE spec.replicas == status.readyReplicas")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "ready" {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestResolveForms(t *testing.T) {
	e, _ := newEngine()
	for _, arg := range []string{"pods", "pod", "deployments.apps", "deployment"} {
		if _, err := e.resolve(arg); err != nil {
			t.Errorf("resolve %q: %v", arg, err)
		}
	}
}

func TestUpdate(t *testing.T) {
	e, c := newEngine(configmap("default", "foobar"))
	res, err := e.ExecuteOne(context.Background(),
		"UPDATE configmap SET data.foo = 'bar' WHERE metadata.namespace = 'default' AND metadata.name = 'foobar'")
	if err != nil {
		t.Fatal(err)
	}
	if res.Affected != 1 {
		t.Fatalf("affected: %d", res.Affected)
	}
	got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	v, _, _ := unstructured.NestedString(got.Object, "data", "foo")
	if v != "bar" {
		t.Errorf("data.foo: %q", v)
	}
}

func TestUpdateMergePatch(t *testing.T) {
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"namespace": "default", "name": "foobar"},
		"data":       map[string]any{"foo": "old", "keep": "yes"},
	}}
	e, c := newEngine(cm)

	var gotPatch string
	c.PrependReactor("patch", "configmaps", func(a clienttesting.Action) (bool, runtime.Object, error) {
		if pa, ok := a.(clienttesting.PatchAction); ok {
			gotPatch = string(pa.GetPatch())
		}
		return false, nil, nil // fall through to the tracker
	})

	res, err := e.ExecuteOne(context.Background(),
		"UPDATE configmaps SET data.foo = 'bar' WHERE metadata.name = 'foobar'")
	if err != nil {
		t.Fatal(err)
	}
	if res.Affected != 1 {
		t.Fatalf("affected: %d", res.Affected)
	}
	// It must be a patch that carries only the SET field, not the whole object.
	if gotPatch == "" {
		t.Fatal("expected a PATCH call, got none")
	}
	if strings.Contains(gotPatch, "keep") {
		t.Errorf("patch should contain only SET fields, got: %s", gotPatch)
	}
	// The untouched field must survive the merge.
	got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, _, _ := unstructured.NestedStringMap(got.Object, "data")
	if data["foo"] != "bar" || data["keep"] != "yes" {
		t.Errorf("merge patch result: %v", data)
	}
}

func TestUpdateNullRemovesField(t *testing.T) {
	e, c := newEngine(configmap("default", "foobar")) // data.foo = "old"
	_, err := e.ExecuteOne(context.Background(),
		"UPDATE configmaps SET data.foo = NULL WHERE metadata.name = 'foobar'")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := unstructured.NestedString(got.Object, "data", "foo"); found {
		t.Errorf("data.foo should have been removed, object: %v", got.Object["data"])
	}
}

func TestUpdateReplicas(t *testing.T) {
	e, c := newEngine(deployment("default", "scaled-down", 0, 0))
	res, err := e.ExecuteOne(context.Background(),
		"UPDATE deployment SET spec.replicas = 1 WHERE metadata.namespace = 'default' AND spec.replicas < 1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Affected != 1 {
		t.Fatalf("affected: %d", res.Affected)
	}
	got, _ := c.Resource(depGVR).Namespace("default").Get(context.Background(), "scaled-down", metav1.GetOptions{})
	v, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if v != 1 {
		t.Errorf("replicas: %d", v)
	}
}

func TestDelete(t *testing.T) {
	e, c := newEngine(
		pod("default", "a", "Running", "n1"),
		pod("default", "b", "Running", "n1"),
		pod("kube-system", "keep", "Running", "n1"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"DELETE FROM pods WHERE metadata.namespace = 'default'")
	if err != nil {
		t.Fatal(err)
	}
	if res.Affected != 2 {
		t.Fatalf("affected: %d", res.Affected)
	}
	left, _ := c.Resource(podGVR).Namespace("default").List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 0 {
		t.Errorf("remaining in default: %d", len(left.Items))
	}
}

func TestAssertEmptyPass(t *testing.T) {
	e, _ := newEngine(pod("default", "a", "Running", "n1"))
	res, err := e.ExecuteOne(context.Background(),
		"ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed')")
	if err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
	if res.Kind != sql.StmtAssert {
		t.Fatalf("kind: %v", res.Kind)
	}
}

func TestAssertEmptyFail(t *testing.T) {
	e, _ := newEngine(
		pod("default", "a", "Failed", "n1"),
		pod("default", "b", "Running", "n1"),
	)
	_, err := e.ExecuteOne(context.Background(),
		"ASSERT EMPTY (SELECT metadata.namespace, metadata.name FROM pods WHERE status.phase = 'Failed') "+
			"FAIL 'FAILED_POD' 'failed pods present'")
	if err == nil {
		t.Fatal("expected a validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "FAILED_POD" || ve.Message != "failed pods present" {
		t.Errorf("code/message: %q / %q", ve.Code, ve.Message)
	}
	if ve.Matched < 1 || len(ve.Sample.Rows) < 1 {
		t.Errorf("expected a sample, matched=%d rows=%d", ve.Matched, len(ve.Sample.Rows))
	}
}

func TestAssertNotEmptyPass(t *testing.T) {
	e, _ := newEngine(deployment("default", "ingress", 1, 1))
	res, err := e.ExecuteOne(context.Background(),
		"ASSERT NOT EMPTY (SELECT metadata.name FROM deployments WHERE metadata.name = 'ingress')")
	if err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
	if res.Affected < 1 {
		t.Errorf("affected: %d", res.Affected)
	}
}

func TestAssertNotEmptyFail(t *testing.T) {
	e, _ := newEngine(pod("default", "a", "Running", "n1"))
	_, err := e.ExecuteOne(context.Background(),
		"ASSERT NOT EMPTY (SELECT metadata.name FROM deployments WHERE metadata.name = 'ingress')")
	if err == nil {
		t.Fatal("expected a validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("type: %T", err)
	}
	if ve.Expect != sql.ExpectNotEmpty {
		t.Errorf("expect: %v", ve.Expect)
	}
	if ve.Matched != 0 {
		t.Errorf("matched: %d", ve.Matched)
	}
}

func TestAssertBatchStopsBeforeMutation(t *testing.T) {
	// A failing ASSERT before a DELETE must prevent the DELETE from running.
	e, c := newEngine(
		pod("default", "a", "Failed", "n1"),
		pod("default", "b", "Running", "n1"),
	)
	_, err := e.Execute(context.Background(),
		"ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed'); "+
			"DELETE FROM pods")
	if err == nil {
		t.Fatal("expected validation error")
	}
	left, _ := c.Resource(podGVR).List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 2 {
		t.Fatalf("DELETE must not run after a failed ASSERT; remaining: %d", len(left.Items))
	}
}

func TestJoin(t *testing.T) {
	e, _ := newEngine(
		pod("default", "web", "Running", "n1"),
		pod("default", "db", "Running", "n2"),
		node("n1", "m5.large"),
		node("n2", "m5.xlarge"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT pod.metadata.name, pod.spec.nodeName, node.metadata.labels.'node.kubernetes.io/instance-type' "+
			"FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %d (%v)", len(res.Rows), res.Rows)
	}
	got := map[string]string{}
	for _, r := range res.Rows {
		got[r[0].(string)] = r[2].(string)
	}
	if got["web"] != "m5.large" || got["db"] != "m5.xlarge" {
		t.Errorf("join result: %v", got)
	}
}

func TestNamespaceInPushdown(t *testing.T) {
	e, c := newEngine(
		pod("ns1", "a", "Running", "n1"),
		pod("ns2", "b", "Running", "n1"),
		pod("ns3", "c", "Running", "n1"),
	)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.namespace IN ('ns1', 'ns3')")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %v", res.Rows)
	}

	// Verify it issued one List per namespace (server-side scoping), not a
	// single all-namespaces scan.
	listNs := map[string]bool{}
	for _, a := range c.Actions() {
		if a.GetVerb() == "list" && a.GetResource() == podGVR {
			listNs[a.GetNamespace()] = true
		}
	}
	if listNs[""] {
		t.Errorf("unexpected all-namespaces list; actions scoped to: %v", listNs)
	}
	if !listNs["ns1"] || !listNs["ns3"] {
		t.Errorf("expected per-namespace lists for ns1 and ns3, got: %v", listNs)
	}
	if listNs["ns2"] {
		t.Errorf("should not have listed ns2: %v", listNs)
	}
}

func TestNamespaceInConcurrent(t *testing.T) {
	// Wide fan-out across many namespaces exercises the bounded-concurrency
	// path; run under -race to catch unsynchronized result handling.
	var objs []runtime.Object
	want := 0
	nsList := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		ns := "ns" + string(rune('a'+i))
		nsList = append(nsList, ns)
		objs = append(objs, pod(ns, "p1", "Running", "n1"), pod(ns, "p2", "Running", "n1"))
		want += 2
	}
	e, _ := newEngine(objs...)

	q := "SELECT metadata.name, metadata.namespace FROM pods WHERE metadata.namespace IN ('" +
		joinStrings(nsList, "','") + "')"
	res, err := e.ExecuteOne(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != want {
		t.Fatalf("expected %d rows, got %d", want, len(res.Rows))
	}
}

func TestListConcurrencyOneSequential(t *testing.T) {
	e, _ := newEngine(
		pod("ns1", "a", "Running", "n1"),
		pod("ns2", "b", "Running", "n1"),
		pod("ns3", "c", "Running", "n1"),
	)
	WithListConcurrency(1)(e)
	res, err := e.ExecuteOne(context.Background(),
		"SELECT metadata.name FROM pods WHERE metadata.namespace IN ('ns1', 'ns2', 'ns3')")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func joinStrings(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

func TestBatchPreparedBeforeExecute(t *testing.T) {
	// The batch deletes pods, then references an unresolvable resource. Because
	// the whole batch is prepared before execution, the DELETE must NOT run.
	e, c := newEngine(
		pod("default", "a", "Running", "n1"),
		pod("default", "b", "Running", "n1"),
	)
	_, err := e.Execute(context.Background(),
		"DELETE FROM pods WHERE metadata.namespace = 'default'; SELECT * FROM bogusresources")
	if err == nil {
		t.Fatal("expected error for unresolvable resource")
	}
	left, _ := c.Resource(podGVR).Namespace("default").List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 2 {
		t.Fatalf("DELETE must not run when a later statement fails to prepare; remaining: %d", len(left.Items))
	}
}

func TestJoinFieldSelectorDisabled(t *testing.T) {
	// With the field-selector pushdown disabled the join must still be correct
	// (full-list fallback path).
	scheme := runtime.NewScheme()
	c := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds(),
		pod("default", "web", "Running", "n1"),
		pod("default", "db", "Running", "n2"),
		node("n1", "m5.large"),
		node("n2", "m5.xlarge"),
	)
	e := New(c, testMapper(), WithMaxFieldSelectorKeys(0))
	res, err := e.ExecuteOne(context.Background(),
		"SELECT pod.metadata.name, node.metadata.labels.'node.kubernetes.io/instance-type' "+
			"FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %d (%v)", len(res.Rows), res.Rows)
	}
}

// platformTable registers the platform facts used by the virtual-table tests.
func platformTable() Option {
	return WithVirtualTable("v_d8_platform", []map[string]any{{
		"deckhouseVersion":  "1.76.9",
		"deckhouseEdition":  "EE",
		"deckhouseBundle":   "Default",
		"kubernetesVersion": "1.31",
	}})
}

func newEngineWith(opts []Option, objs ...runtime.Object) (*Engine, *dynfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	c := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds(), objs...)
	return New(c, testMapper(), opts...), c
}

func TestVirtualTableSelect(t *testing.T) {
	e, c := newEngineWith([]Option{platformTable()})
	res, err := e.ExecuteOne(context.Background(),
		"SELECT deckhouseEdition, kubernetesVersion FROM v_d8_platform")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "EE" || res.Rows[0][1] != "1.31" {
		t.Fatalf("rows: %v", res.Rows)
	}
	if len(c.Actions()) != 0 {
		t.Errorf("a virtual table must not touch the API server, got: %v", c.Actions())
	}
}

func TestVirtualTableWhere(t *testing.T) {
	e, _ := newEngineWith([]Option{platformTable()})
	res, err := e.ExecuteOne(context.Background(),
		"SELECT deckhouseVersion FROM v_d8_platform WHERE deckhouseEdition = 'CE'")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 0 {
		t.Fatalf("rows: %v", res.Rows)
	}
}

func TestVirtualTableAssert(t *testing.T) {
	e, _ := newEngineWith([]Option{platformTable()})
	if _, err := e.ExecuteOne(context.Background(),
		"ASSERT NOT EMPTY (SELECT deckhouseEdition FROM v_d8_platform "+
			"WHERE deckhouseEdition IN ('EE','FE')) FAIL 'EDITION' 'this module requires EE'"); err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}

	_, err := e.ExecuteOne(context.Background(),
		"ASSERT NOT EMPTY (SELECT deckhouseEdition FROM v_d8_platform "+
			"WHERE deckhouseEdition = 'CE') FAIL 'EDITION' 'this module requires CE'")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "EDITION" || ve.Resource != "v_d8_platform" {
		t.Errorf("code/resource: %q / %q", ve.Code, ve.Resource)
	}
}

func TestVirtualTableReadOnly(t *testing.T) {
	e, _ := newEngineWith([]Option{platformTable()}, pod("default", "a", "Running", "n1"))
	cases := []string{
		"UPDATE v_d8_platform SET deckhouseEdition = 'CE'",
		"DELETE FROM v_d8_platform",
		"SELECT p.metadata.name FROM pods p JOIN v_d8_platform v ON p.metadata.name == v.deckhouseEdition",
	}
	for _, q := range cases {
		if _, err := e.ExecuteOne(context.Background(), q); err == nil {
			t.Errorf("expected a rejection for %q", q)
		}
	}
}

func TestIfBranchSelection(t *testing.T) {
	e, c := newEngineWith(nil,
		pod("default", "a", "Running", "n1"),
		configmap("default", "cm"),
	)
	res, err := e.ExecuteOne(context.Background(), `
		IF EXISTS (SELECT * FROM pods WHERE status.phase = 'Failed') THEN
			UPDATE configmaps SET data.foo = 'failed' WHERE metadata.name = 'cm';
		ELSE
			UPDATE configmaps SET data.foo = 'ok' WHERE metadata.name = 'cm';
		END IF`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != sql.StmtIf {
		t.Fatalf("kind: %v", res.Kind)
	}
	if len(res.Nested) != 1 || res.Nested[0].Affected != 1 {
		t.Fatalf("nested results: %+v", res.Nested)
	}
	got, _ := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "cm", metav1.GetOptions{})
	v, _, _ := unstructured.NestedString(got.Object, "data", "foo")
	if v != "ok" {
		t.Errorf("the ELSE branch should have run, data.foo = %q", v)
	}
}

func TestIfElsifOverVirtualTable(t *testing.T) {
	e, c := newEngineWith([]Option{platformTable()}, configmap("default", "cm"))
	_, err := e.ExecuteOne(context.Background(), `
		IF EXISTS (SELECT * FROM v_d8_platform WHERE deckhouseEdition = 'CE') THEN
			UPDATE configmaps SET data.foo = 'ce' WHERE metadata.name = 'cm';
		ELSIF EXISTS (SELECT * FROM v_d8_platform WHERE deckhouseEdition IN ('EE','FE')) THEN
			UPDATE configmaps SET data.foo = 'ee' WHERE metadata.name = 'cm';
		ELSE
			UPDATE configmaps SET data.foo = 'none' WHERE metadata.name = 'cm';
		END IF`)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "cm", metav1.GetOptions{})
	v, _, _ := unstructured.NestedString(got.Object, "data", "foo")
	if v != "ee" {
		t.Errorf("the ELSIF branch should have run, data.foo = %q", v)
	}
}

func TestIfNoBranchMatches(t *testing.T) {
	e, c := newEngineWith(nil, configmap("default", "cm"))
	res, err := e.ExecuteOne(context.Background(), `
		IF EXISTS (SELECT * FROM pods) THEN
			DELETE FROM configmaps;
		END IF`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nested) != 0 {
		t.Errorf("nothing should have run: %+v", res.Nested)
	}
	left, _ := c.Resource(cmGVR).List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 1 {
		t.Errorf("configmap must survive, remaining: %d", len(left.Items))
	}
}

func TestIfNested(t *testing.T) {
	e, c := newEngineWith(nil,
		node("n1", "m5.large"),
		configmap("default", "cm"),
	)
	res, err := e.ExecuteOne(context.Background(), `
		IF EXISTS (SELECT * FROM nodes) THEN
			IF NOT EXISTS (SELECT * FROM pods WHERE status.phase = 'Failed') THEN
				DELETE FROM configmaps WHERE metadata.name = 'cm';
			END IF;
			SELECT metadata.name FROM nodes;
		END IF`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nested) != 2 {
		t.Fatalf("nested results: %+v", res.Nested)
	}
	if res.Nested[0].Kind != sql.StmtIf || len(res.Nested[0].Nested) != 1 {
		t.Errorf("inner IF result: %+v", res.Nested[0])
	}
	if len(res.Nested[1].Rows) != 1 {
		t.Errorf("trailing SELECT: %+v", res.Nested[1].Rows)
	}
	left, _ := c.Resource(cmGVR).List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 0 {
		t.Errorf("the nested DELETE should have run, remaining: %d", len(left.Items))
	}
}

func TestIfAssertPropagates(t *testing.T) {
	e, _ := newEngineWith(nil, pod("default", "a", "Failed", "n1"))
	_, err := e.ExecuteOne(context.Background(), `
		IF NOT EXISTS (SELECT * FROM nodes) THEN
			ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed')
				FAIL 'FAILED_PODS' 'failed pods present';
		END IF`)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "FAILED_PODS" {
		t.Errorf("code: %q", ve.Code)
	}
}

func TestIfPreparedBeforeExecute(t *testing.T) {
	// The unresolvable resource sits in a branch that would never be taken; the
	// batch must still fail before the earlier DELETE runs.
	e, c := newEngineWith(nil, configmap("default", "cm"))
	_, err := e.Execute(context.Background(), `
		DELETE FROM configmaps;
		IF EXISTS (SELECT * FROM pods) THEN
			SELECT * FROM bogusresources;
		END IF;`)
	if err == nil {
		t.Fatal("expected error for the unresolvable resource")
	}
	left, _ := c.Resource(cmGVR).List(context.Background(), metav1.ListOptions{})
	if len(left.Items) != 1 {
		t.Fatalf("DELETE must not run when a branch fails to prepare; remaining: %d", len(left.Items))
	}
}

func TestMultipleStatements(t *testing.T) {
	e, _ := newEngine(
		pod("default", "a", "Running", "n1"),
		node("n1", "m5.large"),
	)
	results, err := e.Execute(context.Background(), "SELECT metadata.name FROM pods; SELECT * FROM nodes")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results: %d", len(results))
	}
}

const insertCM = `INSERT INTO configmaps SET
	metadata.name = 'foobar',
	metadata.namespace = 'default',
	data.greeting = 'hello'`

func TestInsert(t *testing.T) {
	e, c := newEngine()
	res, err := e.ExecuteOne(context.Background(), insertCM)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != sql.StmtInsert || res.Affected != 1 || len(res.Objects) != 1 {
		t.Fatalf("result: %+v", res)
	}
	got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetAPIVersion() != "v1" || got.GetKind() != "ConfigMap" {
		t.Errorf("apiVersion/kind: %q / %q", got.GetAPIVersion(), got.GetKind())
	}
	if got.GetNamespace() != "default" || got.GetName() != "foobar" {
		t.Errorf("namespace/name: %q / %q", got.GetNamespace(), got.GetName())
	}
	if v, _, _ := unstructured.NestedString(got.Object, "data", "greeting"); v != "hello" {
		t.Errorf("data.greeting: %q", v)
	}
}

func TestInsertDefaultNamespace(t *testing.T) {
	e, c := newEngineWith([]Option{WithDefaultNamespace("d8-system")})
	if _, err := e.ExecuteOne(context.Background(),
		"INSERT INTO configmaps SET metadata.name = 'foobar', data.greeting = 'hello'"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Resource(cmGVR).Namespace("d8-system").Get(
		context.Background(), "foobar", metav1.GetOptions{}); err != nil {
		t.Fatalf("object should have landed in the default namespace: %v", err)
	}
}

func TestInsertRejected(t *testing.T) {
	cases := []struct {
		name  string
		opts  []Option
		query string
	}{
		{
			name:  "no namespace anywhere",
			query: "INSERT INTO configmaps SET metadata.name = 'foobar'",
		},
		{
			name:  "namespace on a cluster-scoped resource",
			query: "INSERT INTO nodes SET metadata.name = 'n1', metadata.namespace = 'default'",
		},
		{
			name:  "missing metadata.name",
			query: "INSERT INTO configmaps SET metadata.namespace = 'default', data.greeting = 'hello'",
		},
		{
			name:  "non-string metadata.name",
			query: "INSERT INTO configmaps SET metadata.name = 42, metadata.namespace = 'default'",
		},
		{
			name:  "virtual table",
			opts:  []Option{platformTable()},
			query: "INSERT INTO v_d8_platform SET deckhouseEdition = 'CE'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, c := newEngineWith(tc.opts)
			if _, err := e.ExecuteOne(context.Background(), tc.query); err == nil {
				t.Fatalf("expected a rejection for %q", tc.query)
			}
			for _, a := range c.Actions() {
				if a.GetVerb() == "create" {
					t.Errorf("nothing must be created, got: %v", c.Actions())
				}
			}
		})
	}
}

func TestInsertAlreadyExistsPropagates(t *testing.T) {
	e, _ := newEngine(configmap("default", "foobar"))
	_, err := e.ExecuteOne(context.Background(), insertCM)
	if err == nil {
		t.Fatal("expected an AlreadyExists error")
	}
	if !apierrors.IsAlreadyExists(err) {
		t.Fatalf("expected an AlreadyExists error, got %T: %v", err, err)
	}
}

func TestInsertIfNotExists(t *testing.T) {
	const query = `
		IF NOT EXISTS (SELECT * FROM configmaps WHERE metadata.namespace = 'default' AND metadata.name = 'foobar') THEN
			INSERT INTO configmaps SET metadata.name = 'foobar', metadata.namespace = 'default', data.greeting = 'hello';
		END IF`

	t.Run("creates when missing", func(t *testing.T) {
		e, c := newEngine()
		res, err := e.ExecuteOne(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Nested) != 1 || res.Nested[0].Affected != 1 {
			t.Fatalf("nested results: %+v", res.Nested)
		}
		got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if v, _, _ := unstructured.NestedString(got.Object, "data", "greeting"); v != "hello" {
			t.Errorf("data.greeting: %q", v)
		}
	})

	t.Run("no-op when present", func(t *testing.T) {
		e, c := newEngine(configmap("default", "foobar")) // data.foo = "old"
		res, err := e.ExecuteOne(context.Background(), query)
		if err != nil {
			t.Fatalf("the INSERT must be skipped, not attempted: %v", err)
		}
		if len(res.Nested) != 0 {
			t.Errorf("nothing should have run: %+v", res.Nested)
		}
		got, err := c.Resource(cmGVR).Namespace("default").Get(context.Background(), "foobar", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if v, _, _ := unstructured.NestedString(got.Object, "data", "foo"); v != "old" {
			t.Errorf("the existing object must be left alone, data.foo = %q", v)
		}
	})
}
