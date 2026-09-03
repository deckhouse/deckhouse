package d8sql

import (
	"encoding/json"
	"testing"

	"github.com/deckhouse/d8sql/sql"
)

func TestMergePatchData(t *testing.T) {
	st, err := sql.Parse("UPDATE configmap SET data.foo = 'bar', data.gone = NULL, spec.replicas = 3 " +
		"WHERE metadata.name = 'x'")
	if err != nil {
		t.Fatal(err)
	}
	data, err := mergePatchData("42", st.Assignments)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta == nil || meta["resourceVersion"] != "42" {
		t.Errorf("resourceVersion not embedded: %v", m["metadata"])
	}
	d, _ := m["data"].(map[string]any)
	if d["foo"] != "bar" {
		t.Errorf("data.foo: %v", d["foo"])
	}
	if v, present := d["gone"]; !present || v != nil {
		t.Errorf("data.gone should be JSON null (remove): present=%v v=%v", present, v)
	}
	if sp, _ := m["spec"].(map[string]any); sp == nil || sp["replicas"].(float64) != 3 {
		t.Errorf("spec.replicas: %v", m["spec"])
	}

	// With no resourceVersion, none is embedded (no precondition possible).
	data2, err := mergePatchData("", st.Assignments)
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := json.Unmarshal(data2, &m2); err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["metadata"]; ok {
		t.Errorf("metadata must be absent without resourceVersion: %v", m2["metadata"])
	}
}

func TestLikeMatcher(t *testing.T) {
	cases := []struct {
		pat, s string
		ci     bool
		want   bool
	}{
		{"%redis%", "my-redis-master", false, true},
		{"%redis%", "postgres", false, false},
		{"redis%", "redis-0", false, true},
		{"redis%", "my-redis", false, false},
		{"%master", "redis-master", false, true},
		{"r_dis", "redis", false, true},
		{"r_dis", "rdis", false, false},
		{"exact", "exact", false, true},
		{"exact", "Exact", false, false},
		{"exact", "Exact", true, true},
		{"%REDIS%", "my-redis-master", true, true},
		{"%", "anything", false, true},
		{"", "", false, true},
		{"", "x", false, false},
	}
	for _, c := range cases {
		m := compileLike(c.pat, c.ci)
		if got := m(c.s); got != c.want {
			t.Errorf("LIKE %q ci=%v on %q: got %v want %v", c.pat, c.ci, c.s, got, c.want)
		}
	}
}

func TestGetPath(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"name": "foo",
			"labels": map[string]any{
				"node.kubernetes.io/instance-type": "m5.large",
			},
		},
		"spec": map[string]any{"replicas": int64(3)},
	}
	if v, ok := getPath(obj, []string{"metadata", "name"}); !ok || v != "foo" {
		t.Errorf("name: %v %v", v, ok)
	}
	if v, ok := getPath(obj, []string{"metadata", "labels", "node.kubernetes.io/instance-type"}); !ok || v != "m5.large" {
		t.Errorf("label: %v %v", v, ok)
	}
	if _, ok := getPath(obj, []string{"status", "phase"}); ok {
		t.Error("expected missing")
	}
	if v, ok := getPath(obj, []string{"spec", "replicas"}); !ok || v != int64(3) {
		t.Errorf("replicas: %v %v", v, ok)
	}
}

func TestLabelSelectorFor(t *testing.T) {
	st, err := sql.Parse("SELECT pod.metadata.name FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name " +
		"WHERE node.metadata.labels.'node.deckhouse.io/group' = 'worker' AND pod.status.phase = 'Running'")
	if err != nil {
		t.Fatal(err)
	}
	if got := labelSelectorFor(st.Where, "node", true); got != "node.deckhouse.io/group=worker" {
		t.Errorf("node selector: %q", got)
	}
	if got := labelSelectorFor(st.Where, "pod", true); got != "" {
		t.Errorf("pod selector should be empty, got %q", got)
	}
}

func TestLabelSelectorSetBased(t *testing.T) {
	cases := []struct {
		where string
		want  string
	}{
		{"metadata.labels.'app' != 'redis'", "app!=redis"},
		{"metadata.labels.'app' IN ('redis', 'memcached')", "app in (memcached,redis)"},
		{"metadata.labels.'app' NOT IN ('redis')", "app notin (redis)"},
		{"metadata.labels.'app' IS NOT NULL", "app"},
		{"metadata.labels.'app' IS NULL", "!app"},
		// disjunction must not be pushed down
		{"metadata.labels.'app' = 'a' OR metadata.labels.'app' = 'b'", ""},
	}
	for _, c := range cases {
		st, err := sql.Parse("SELECT * FROM pods WHERE " + c.where)
		if err != nil {
			t.Fatalf("%s: %v", c.where, err)
		}
		if got := labelSelectorFor(st.Where, "pods", true); got != c.want {
			t.Errorf("%s: got %q want %q", c.where, got, c.want)
		}
	}
}

func TestLabelSelectorMultiple(t *testing.T) {
	st, err := sql.Parse("SELECT * FROM pods WHERE metadata.labels.'app' = 'redis' AND metadata.labels.'tier' = 'cache'")
	if err != nil {
		t.Fatal(err)
	}
	// unqualified (single-table) fields use empty alias
	got := labelSelectorFor(st.Where, "", true)
	if got != "app=redis,tier=cache" {
		t.Errorf("selector: %q", got)
	}
}

func TestDotPath(t *testing.T) {
	if got := dotPath([]string{"spec", "nodeName"}); got != "spec.nodeName" {
		t.Errorf("dotPath: %q", got)
	}
}

func TestCompareValues(t *testing.T) {
	if !valueEqual(int64(3), float64(3)) {
		t.Error("int64 vs float64 equality")
	}
	if !compareValues(sql.LT, int64(1), int64(2)) {
		t.Error("1 < 2")
	}
	if compareValues(sql.GT, int64(2), int64(2)) {
		t.Error("2 > 2 should be false")
	}
	if !valueEqual("a", "a") {
		t.Error("string equality")
	}
}
