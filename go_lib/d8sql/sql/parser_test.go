package sql

import "testing"

func TestParseSelectProjection(t *testing.T) {
	st, err := Parse("SELECT metadata.name, metadata.namespace FROM pods WHERE status.phase = 'Running'")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtSelect {
		t.Fatalf("kind: %v", st.Kind)
	}
	if st.Star {
		t.Fatal("unexpected star")
	}
	if len(st.Columns) != 2 {
		t.Fatalf("columns: %d", len(st.Columns))
	}
	if st.Columns[0].Field.String() != "metadata.name" {
		t.Errorf("col0: %q", st.Columns[0].Field.String())
	}
	if st.Table.Resource != "pods" {
		t.Errorf("table: %q", st.Table.Resource)
	}
	cmp, ok := st.Where.(*Compare)
	if !ok || cmp.Op != EQ {
		t.Fatalf("where: %T", st.Where)
	}
}

func TestParseSelectStar(t *testing.T) {
	st, err := Parse("SELECT * FROM nodes")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Star {
		t.Fatal("want star")
	}
	if st.Table.Resource != "nodes" {
		t.Errorf("table: %q", st.Table.Resource)
	}
}

func TestParseQuotedResource(t *testing.T) {
	st, err := Parse("SELECT metadata.name FROM 'envoyproxies.gateway.envoyproxy.io'")
	if err != nil {
		t.Fatal(err)
	}
	if st.Table.Resource != "envoyproxies.gateway.envoyproxy.io" {
		t.Errorf("table: %q", st.Table.Resource)
	}
}

func TestParseGroupQualifiedResource(t *testing.T) {
	st, err := Parse("SELECT metadata.name FROM rolebindings.rbac.authorization.k8s.io")
	if err != nil {
		t.Fatal(err)
	}
	if st.Table.Resource != "rolebindings.rbac.authorization.k8s.io" {
		t.Errorf("table: %q", st.Table.Resource)
	}
}

func TestParseLike(t *testing.T) {
	st, err := Parse("SELECT metadata.name FROM services WHERE metadata.name LIKE '%redis%'")
	if err != nil {
		t.Fatal(err)
	}
	cmp, ok := st.Where.(*Compare)
	if !ok || cmp.Op != LIKE {
		t.Fatalf("where: %T", st.Where)
	}
	lit := cmp.Right.(*Lit)
	if lit.Str != "%redis%" {
		t.Errorf("pattern: %q", lit.Str)
	}
}

func TestParseFieldToField(t *testing.T) {
	st, err := Parse("SELECT metadata.name FROM deployments WHERE spec.replicas == status.readyReplicas")
	if err != nil {
		t.Fatal(err)
	}
	cmp := st.Where.(*Compare)
	if _, ok := cmp.Left.(*Field); !ok {
		t.Errorf("left not field: %T", cmp.Left)
	}
	if _, ok := cmp.Right.(*Field); !ok {
		t.Errorf("right not field: %T", cmp.Right)
	}
}

func TestParseJoin(t *testing.T) {
	st, err := Parse("SELECT pod.metadata.name, node.metadata.labels.'node.kubernetes.io/instance-type' FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name")
	if err != nil {
		t.Fatal(err)
	}
	if st.Join == nil {
		t.Fatal("no join")
	}
	if st.Table.Resource != "pod" || st.Join.Right.Resource != "node" {
		t.Errorf("tables: %q %q", st.Table.Resource, st.Join.Right.Resource)
	}
	// after resolveJoinFields the table prefix is split out
	if st.Join.On.Left.Table != "pod" || want(st.Join.On.Left.Path, "spec", "nodeName") == false {
		t.Errorf("join left: %+v", st.Join.On.Left)
	}
	if st.Join.On.Right.Table != "node" || want(st.Join.On.Right.Path, "metadata", "name") == false {
		t.Errorf("join right: %+v", st.Join.On.Right)
	}
	// labels column with quoted segment
	c1 := st.Columns[1].Field
	if c1.Table != "node" || want(c1.Path, "metadata", "labels", "node.kubernetes.io/instance-type") == false {
		t.Errorf("labels col: %+v", c1)
	}
}

func TestParseInsert(t *testing.T) {
	st, err := Parse("INSERT INTO configmaps SET metadata.name = 'foobar', data.greeting = 'hello', data.count = 1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtInsert {
		t.Fatalf("kind: %v", st.Kind)
	}
	if st.Table.Resource != "configmaps" {
		t.Errorf("table: %q", st.Table.Resource)
	}
	if st.Where != nil {
		t.Errorf("INSERT must not carry a WHERE: %T", st.Where)
	}
	if len(st.Assignments) != 3 {
		t.Fatalf("assignments: %d", len(st.Assignments))
	}
	if !want(st.Assignments[0].Field.Path, "metadata", "name") || st.Assignments[0].Value.Str != "foobar" {
		t.Errorf("assignment 0: %+v", st.Assignments[0])
	}
	if !want(st.Assignments[1].Field.Path, "data", "greeting") || st.Assignments[1].Value.Str != "hello" {
		t.Errorf("assignment 1: %+v", st.Assignments[1])
	}
	if st.Assignments[2].Value.Kind != LitInt || st.Assignments[2].Value.Int != 1 {
		t.Errorf("assignment 2: %+v", st.Assignments[2].Value)
	}
}

func TestParseInsertErrors(t *testing.T) {
	cases := []string{
		"INSERT configmaps SET metadata.name = 'x'",                                // missing INTO
		"INSERT INTO configmaps metadata.name = 'x'",                               // missing SET
		"INSERT INTO configmaps SET",                                               // empty assignment list
		"INSERT INTO configmaps SET metadata.name",                                 // assignment without a value
		"INSERT INTO configmaps SET metadata.name = 'x' WHERE metadata.name = 'x'", // WHERE is not allowed
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseUpdate(t *testing.T) {
	st, err := Parse("UPDATE configmap SET data.foo = 'bar' WHERE metadata.namespace = 'default' AND metadata.name = 'foobar'")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtUpdate {
		t.Fatalf("kind: %v", st.Kind)
	}
	if len(st.Assignments) != 1 {
		t.Fatalf("assignments: %d", len(st.Assignments))
	}
	if st.Assignments[0].Value.Str != "bar" {
		t.Errorf("value: %q", st.Assignments[0].Value.Str)
	}
}

func TestParseUpdateInt(t *testing.T) {
	st, err := Parse("UPDATE deployment SET spec.replicas = 1 WHERE spec.replicas < 1")
	if err != nil {
		t.Fatal(err)
	}
	a := st.Assignments[0]
	if a.Value.Kind != LitInt || a.Value.Int != 1 {
		t.Errorf("value: %+v", a.Value)
	}
}

func TestParseDelete(t *testing.T) {
	st, err := Parse("DELETE FROM pods WHERE metadata.namespace = 'default'")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtDelete {
		t.Fatalf("kind: %v", st.Kind)
	}
	if st.Table.Resource != "pods" {
		t.Errorf("table: %q", st.Table.Resource)
	}
}

func TestParseAssertEmpty(t *testing.T) {
	st, err := Parse("ASSERT EMPTY (SELECT metadata.name FROM ingressnginxcontrollers.deckhouse.io " +
		"WHERE spec.controllerVersion = '1.12') FAIL 'NGINX_1_12' 'controllers on 1.12 found'")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtAssert {
		t.Fatalf("kind: %v", st.Kind)
	}
	if st.Assert == nil {
		t.Fatal("nil assert clause")
	}
	if st.Assert.Expect != ExpectEmpty {
		t.Errorf("expect: %v", st.Assert.Expect)
	}
	if st.Assert.Code != "NGINX_1_12" || st.Assert.Message != "controllers on 1.12 found" {
		t.Errorf("code/message: %q / %q", st.Assert.Code, st.Assert.Message)
	}
	inner := st.Assert.Select
	if inner == nil || inner.Kind != StmtSelect {
		t.Fatalf("inner select: %+v", inner)
	}
	if inner.Table.Resource != "ingressnginxcontrollers.deckhouse.io" {
		t.Errorf("inner table: %q", inner.Table.Resource)
	}
	if _, ok := inner.Where.(*Compare); !ok {
		t.Errorf("inner where: %T", inner.Where)
	}
}

func TestParseAssertNotEmpty(t *testing.T) {
	st, err := Parse("ASSERT NOT EMPTY (SELECT metadata.name FROM deployments WHERE metadata.name = 'ingress')")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtAssert || st.Assert.Expect != ExpectNotEmpty {
		t.Fatalf("kind/expect: %v / %v", st.Kind, st.Assert.Expect)
	}
	if st.Assert.Code != "" || st.Assert.Message != "" {
		t.Errorf("fail clause should be empty: %q / %q", st.Assert.Code, st.Assert.Message)
	}
}

func TestParseAssertErrors(t *testing.T) {
	cases := []string{
		"ASSERT (SELECT * FROM pods)",            // missing EMPTY / NOT EMPTY
		"ASSERT EMPTY SELECT * FROM pods",        // missing parentheses
		"ASSERT EMPTY (DELETE FROM pods)",        // inner must be SELECT
		"ASSERT EMPTY (SELECT * FROM pods) FAIL", // FAIL requires a code string
		"ASSERT NOT (SELECT * FROM pods)",        // NOT must be followed by EMPTY
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseMany(t *testing.T) {
	stmts, err := ParseMany("SELECT * FROM pods; SELECT * FROM nodes;")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 2 {
		t.Fatalf("stmts: %d", len(stmts))
	}
}

func TestParseErrors(t *testing.T) {
	cases := []string{
		"SELECT FROM pods",
		"SELECT * pods",
		"UPDATE pods SET WHERE x = 1",
		"DELETE pods",
		"",
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseIf(t *testing.T) {
	st, err := Parse("IF EXISTS (SELECT * FROM v_d8_platform WHERE deckhouseEdition = 'EE') THEN " +
		"UPDATE moduleconfigs.deckhouse.io SET spec.enabled = true WHERE metadata.name = 'foo'; " +
		"END IF")
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != StmtIf || st.If == nil {
		t.Fatalf("kind: %v, clause: %+v", st.Kind, st.If)
	}
	if len(st.If.Branches) != 1 || st.If.Else != nil {
		t.Fatalf("branches: %d, else: %v", len(st.If.Branches), st.If.Else)
	}
	ex, ok := st.If.Branches[0].Cond.(*Exists)
	if !ok {
		t.Fatalf("cond: %T", st.If.Branches[0].Cond)
	}
	if ex.Select.Table.Resource != "v_d8_platform" {
		t.Errorf("exists table: %q", ex.Select.Table.Resource)
	}
	body := st.If.Branches[0].Body
	if len(body) != 1 || body[0].Kind != StmtUpdate {
		t.Fatalf("body: %+v", body)
	}
}

func TestParseIfElsifElse(t *testing.T) {
	st, err := Parse(`
		IF NOT EXISTS (SELECT * FROM pods WHERE status.phase = 'Failed') THEN
			SELECT metadata.name FROM pods;
		ELSIF EXISTS (SELECT * FROM nodes) AND EXISTS (SELECT * FROM configmaps) THEN
			ASSERT EMPTY (SELECT * FROM pods) FAIL 'X' 'msg';
			DELETE FROM configmaps WHERE metadata.name = 'old';
		ELSE
			DELETE FROM pods WHERE metadata.namespace = 'tmp';
		END IF;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.If.Branches) != 2 {
		t.Fatalf("branches: %d", len(st.If.Branches))
	}
	if _, ok := st.If.Branches[0].Cond.(*Not); !ok {
		t.Errorf("branch 0 cond: %T", st.If.Branches[0].Cond)
	}
	lg, ok := st.If.Branches[1].Cond.(*Logic)
	if !ok || lg.Op != AND {
		t.Fatalf("branch 1 cond: %T", st.If.Branches[1].Cond)
	}
	if len(st.If.Branches[1].Body) != 2 {
		t.Errorf("branch 1 body: %d", len(st.If.Branches[1].Body))
	}
	if len(st.If.Else) != 1 || st.If.Else[0].Kind != StmtDelete {
		t.Errorf("else: %+v", st.If.Else)
	}
}

func TestParseIfNested(t *testing.T) {
	st, err := Parse(`
		IF EXISTS (SELECT * FROM nodes) THEN
			IF NOT EXISTS (SELECT * FROM pods) THEN
				DELETE FROM configmaps;
			END IF;
			SELECT * FROM pods;
		END IF`)
	if err != nil {
		t.Fatal(err)
	}
	body := st.If.Branches[0].Body
	if len(body) != 2 {
		t.Fatalf("body: %d", len(body))
	}
	if body[0].Kind != StmtIf || body[0].If == nil {
		t.Fatalf("nested statement: %+v", body[0])
	}
	if body[1].Kind != StmtSelect {
		t.Errorf("second statement: %v", body[1].Kind)
	}
}

func TestParseIfOrParens(t *testing.T) {
	st, err := Parse("IF (EXISTS (SELECT * FROM pods) OR NOT EXISTS (SELECT * FROM nodes)) THEN " +
		"SELECT * FROM pods; END IF")
	if err != nil {
		t.Fatal(err)
	}
	lg, ok := st.If.Branches[0].Cond.(*Logic)
	if !ok || lg.Op != OR {
		t.Fatalf("cond: %T", st.If.Branches[0].Cond)
	}
}

func TestParseIfErrors(t *testing.T) {
	cases := []string{
		"IF EXISTS (SELECT * FROM pods) THEN SELECT * FROM pods;",         // missing END IF
		"IF EXISTS (SELECT * FROM pods) SELECT * FROM pods; END IF",       // missing THEN
		"IF THEN SELECT * FROM pods; END IF",                              // missing condition
		"IF EXISTS (DELETE FROM pods) THEN SELECT * FROM pods; END IF",    // EXISTS needs a SELECT
		"IF EXISTS SELECT * FROM pods THEN SELECT * FROM pods; END IF",    // EXISTS needs parentheses
		"IF EXISTS (SELECT * FROM pods) THEN SELECT * FROM pods; END",     // END without IF
		"IF EXISTS (SELECT * FROM pods) THEN SELECT * FROM pods END IF;;", // stray input after END IF
		"ELSE SELECT * FROM pods; END IF",                                 // ELSE without IF
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseKeywordFieldSegment(t *testing.T) {
	// Reserved words are still usable as object field names.
	st, err := Parse("SELECT status.end FROM pods WHERE spec.if = 'yes'")
	if err != nil {
		t.Fatal(err)
	}
	if !want(st.Columns[0].Field.Path, "status", "end") {
		t.Errorf("column path: %v", st.Columns[0].Field.Path)
	}
	cmp, ok := st.Where.(*Compare)
	if !ok {
		t.Fatalf("where: %T", st.Where)
	}
	if !want(cmp.Left.(*Field).Path, "spec", "if") {
		t.Errorf("where path: %v", cmp.Left.(*Field).Path)
	}
}

func want(path []string, segs ...string) bool {
	if len(path) != len(segs) {
		return false
	}
	for i := range segs {
		if path[i] != segs[i] {
			return false
		}
	}
	return true
}
