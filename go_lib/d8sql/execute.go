package d8sql

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/deckhouse/d8sql/sql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/retry"
)

// plan is a fully prepared statement: resources are resolved, the WHERE clause
// and projection are compiled, and namespace/name pushdown is computed. Building
// a plan performs no mutating cluster operations, so a whole batch can be
// prepared (and validated) up front before any statement runs.
type plan struct {
	kind sql.StmtKind
	st   *sql.Statement

	res  resolved
	pred predicate

	// namespaces is the pushed-down set of namespaces to list. nil/empty means
	// "all namespaces" (a single cluster-wide List). A single entry lists one
	// namespace; multiple entries (from metadata.namespace IN (...)) issue one
	// List per namespace, avoiding a full-cluster scan.
	namespaces []string

	hasName bool   // metadata.name pushdown available (GET instead of LIST)
	getName string

	// labelSel is a server-side label selector derived from metadata.labels.*
	// constraints in WHERE (single-table queries). It only narrows the fetched
	// set; the compiled predicate still re-checks every object client-side.
	labelSel string

	// projection (SELECT, non-star)
	star    bool
	getters []getter
	headers []string

	// join
	isJoin   bool
	rightRes resolved
	leftKey  getter
	rightKey getter

	// join pushdown: label selectors per side (server-side filter) and the
	// join-key field paths used for field-selector pushdown on the probe side.
	leftLabelSel  string
	rightLabelSel string
	leftKeyPath   []string
	rightKeyPath  []string

	// virtual table (SELECT only): rows served from memory instead of the API
	// server. isVirtual distinguishes a registered but empty table from a real
	// resource.
	isVirtual   bool
	virtualRows []map[string]any

	// assert (StmtAssert): the inner SELECT plan plus the expectation and the
	// optional typed failure.
	assertInner *plan
	expect      sql.ExpectKind
	failCode    string
	failMessage string

	// if (StmtIf): the compiled branches in source order plus the optional ELSE
	// body. Every branch is compiled during prepare, so a broken statement in a
	// branch that is never taken still fails the batch before anything runs.
	ifBranches []ifBranch
	ifElse     []*plan
}

// ifBranch is one prepared IF/ELSIF arm.
type ifBranch struct {
	cond condition
	body []*plan
}

// condition is a compiled IF condition. It queries the cluster (EXISTS terms are
// streamed and stopped at the first match), so it returns an error for genuine
// failures; "no match" is simply false.
type condition func(ctx context.Context) (bool, error)

// prepare resolves and compiles a single statement without touching the cluster
// state mutably. Resource resolution may consult the (cached) REST mapper.
func (e *Engine) prepare(st *sql.Statement) (*plan, error) {
	switch st.Kind {
	case sql.StmtSelect:
		if st.Join != nil {
			return e.prepareJoin(st)
		}
		return e.prepareSingle(st)
	case sql.StmtUpdate, sql.StmtDelete:
		return e.prepareSingle(st)
	case sql.StmtAssert:
		return e.prepareAssert(st)
	case sql.StmtIf:
		return e.prepareIf(st)
	default:
		return nil, errf("unsupported statement kind")
	}
}

// prepareIf compiles every branch condition and every body statement of an IF,
// including branches that will not be taken, so the whole batch is still
// validated before the first statement runs.
func (e *Engine) prepareIf(st *sql.Statement) (*plan, error) {
	if st.If == nil || len(st.If.Branches) == 0 {
		return nil, errf("IF requires at least one branch")
	}
	p := &plan{kind: sql.StmtIf, st: st, ifBranches: make([]ifBranch, 0, len(st.If.Branches))}
	for _, b := range st.If.Branches {
		cond, err := e.compileCondition(b.Cond)
		if err != nil {
			return nil, err
		}
		body, err := e.prepareBody(b.Body)
		if err != nil {
			return nil, err
		}
		p.ifBranches = append(p.ifBranches, ifBranch{cond: cond, body: body})
	}
	els, err := e.prepareBody(st.If.Else)
	if err != nil {
		return nil, err
	}
	p.ifElse = els
	return p, nil
}

func (e *Engine) prepareBody(stmts []*sql.Statement) ([]*plan, error) {
	if len(stmts) == 0 {
		return nil, nil
	}
	plans := make([]*plan, len(stmts))
	for i, st := range stmts {
		p, err := e.prepare(st)
		if err != nil {
			return nil, err
		}
		plans[i] = p
	}
	return plans, nil
}

// compileCondition turns an IF condition into a closure, mirroring how WHERE
// clauses are compiled: the AST is walked once, at prepare time, and evaluation
// is a plain call. AND/OR short-circuit, so a decided condition issues no
// further List calls.
func (e *Engine) compileCondition(x sql.Expr) (condition, error) {
	switch n := x.(type) {
	case *sql.Logic:
		l, err := e.compileCondition(n.Left)
		if err != nil {
			return nil, err
		}
		r, err := e.compileCondition(n.Right)
		if err != nil {
			return nil, err
		}
		if n.Op == sql.AND {
			return func(ctx context.Context) (bool, error) {
				v, err := l(ctx)
				if err != nil || !v {
					return false, err
				}
				return r(ctx)
			}, nil
		}
		return func(ctx context.Context) (bool, error) {
			v, err := l(ctx)
			if err != nil || v {
				return v, err
			}
			return r(ctx)
		}, nil
	case *sql.Not:
		inner, err := e.compileCondition(n.X)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context) (bool, error) {
			v, err := inner(ctx)
			if err != nil {
				return false, err
			}
			return !v, nil
		}, nil
	case *sql.Exists:
		inner, err := e.prepare(n.Select)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context) (bool, error) { return e.exists(ctx, inner) }, nil
	default:
		return nil, errf("unsupported expression %T in IF condition; expected [NOT] EXISTS (SELECT ...)", x)
	}
}

// prepareAssert prepares the inner SELECT and records the expectation. It mutates
// nothing, so it participates in the same up-front batch validation as the rest.
func (e *Engine) prepareAssert(st *sql.Statement) (*plan, error) {
	if st.Assert == nil || st.Assert.Select == nil {
		return nil, errf("ASSERT requires a SELECT subquery")
	}
	inner, err := e.prepare(st.Assert.Select)
	if err != nil {
		return nil, err
	}
	return &plan{
		kind:        sql.StmtAssert,
		st:          st,
		assertInner: inner,
		expect:      st.Assert.Expect,
		failCode:    st.Assert.Code,
		failMessage: st.Assert.Message,
	}, nil
}

func (e *Engine) prepareSingle(st *sql.Statement) (*plan, error) {
	if rows, ok := e.virtual[st.Table.Resource]; ok {
		if st.Kind != sql.StmtSelect {
			return nil, errf("virtual table %q is read-only", st.Table.Resource)
		}
		return e.prepareVirtual(st, rows)
	}
	res, err := e.resolve(st.Table.Resource)
	if err != nil {
		return nil, err
	}
	var c compiler
	pred, err := c.compilePredicate(st.Where)
	if err != nil {
		return nil, err
	}
	p := &plan{kind: st.Kind, st: st, res: res, pred: pred, star: st.Star}
	if res.namespaced {
		if nss, ok := extractNamespaces(st.Where); ok {
			p.namespaces = nss
		} else if e.defaultNS != NamespaceAll {
			p.namespaces = []string{e.defaultNS}
		}
		// otherwise nil => all namespaces
	}
	if n, ok := extractName(st.Where); ok {
		p.hasName = true
		p.getName = n
	}
	// Push metadata.labels.* equalities/sets down to the API server. The single
	// table is referenced either unqualified or by its alias/resource name.
	tableAlias := st.Table.Alias
	if tableAlias == "" {
		tableAlias = st.Table.Resource
	}
	p.labelSel = labelSelectorFor(st.Where, tableAlias, true)
	if st.Kind == sql.StmtSelect && !st.Star {
		p.getters, p.headers = c.projection(st)
	}
	return p, nil
}

// prepareVirtual compiles a SELECT over a registered in-memory table. There is
// nothing to resolve or push down — the rows are already in memory — so only the
// WHERE predicate and the projection are compiled.
func (e *Engine) prepareVirtual(st *sql.Statement, rows []map[string]any) (*plan, error) {
	var c compiler
	pred, err := c.compilePredicate(st.Where)
	if err != nil {
		return nil, err
	}
	p := &plan{kind: sql.StmtSelect, st: st, pred: pred, star: st.Star, isVirtual: true, virtualRows: rows}
	if !st.Star {
		p.getters, p.headers = c.projection(st)
	}
	return p, nil
}

func (e *Engine) prepareJoin(st *sql.Statement) (*plan, error) {
	for _, name := range [...]string{st.Table.Resource, st.Join.Right.Resource} {
		if _, ok := e.virtual[name]; ok {
			return nil, errf("virtual table %q cannot be joined", name)
		}
	}
	leftRes, err := e.resolve(st.Table.Resource)
	if err != nil {
		return nil, err
	}
	rightRes, err := e.resolve(st.Join.Right.Resource)
	if err != nil {
		return nil, err
	}

	leftAlias := st.Table.Alias
	if leftAlias == "" {
		leftAlias = st.Table.Resource
	}
	rightAlias := st.Join.Right.Alias
	if rightAlias == "" {
		rightAlias = st.Join.Right.Resource
	}
	c := compiler{leftAlias: leftAlias, rightAlias: rightAlias}

	leftKey := c.compileField(st.Join.On.Left)
	rightKey := c.compileField(st.Join.On.Right)
	pred, err := c.compilePredicate(st.Where)
	if err != nil {
		return nil, err
	}
	getters, headers := c.projection(st)

	// Joins span the whole cluster: namespaced sides are listed across all
	// namespaces (ns left empty) rather than defaulting to a single namespace,
	// so e.g. "all pods on nodes with label X" works regardless of namespace.
	// A WHERE clause can still narrow results (including by metadata.namespace).
	p := &plan{
		kind:          sql.StmtSelect,
		st:            st,
		isJoin:        true,
		res:           leftRes,
		rightRes:      rightRes,
		pred:          pred,
		leftKey:       leftKey,
		rightKey:      rightKey,
		getters:       getters,
		headers:       headers,
		// Unqualified label conditions default to the left side (mirroring
		// compileField), so only the left side accepts them; pushing them to the
		// right could wrongly drop join rows.
		leftLabelSel:  labelSelectorFor(st.Where, leftAlias, true),
		rightLabelSel: labelSelectorFor(st.Where, rightAlias, false),
		leftKeyPath:   st.Join.On.Left.Path,
		rightKeyPath:  st.Join.On.Right.Path,
	}
	return p, nil
}

// run executes a prepared plan against the cluster.
func (e *Engine) run(ctx context.Context, p *plan) (Result, error) {
	switch p.kind {
	case sql.StmtSelect:
		if p.isJoin {
			return e.runJoin(ctx, p)
		}
		return e.runSelect(ctx, p)
	case sql.StmtUpdate:
		return e.runUpdate(ctx, p)
	case sql.StmtDelete:
		return e.runDelete(ctx, p)
	case sql.StmtAssert:
		return e.runAssert(ctx, p)
	case sql.StmtIf:
		return e.runIf(ctx, p)
	default:
		return Result{}, errf("unsupported statement kind")
	}
}

// runIf evaluates the branch conditions in order and runs the body of the first
// one that holds, falling back to the ELSE body. The results of the statements
// that ran are returned nested in Result.Nested; an error (including a
// *ValidationError from an ASSERT inside the branch) stops the body and
// propagates to the caller.
func (e *Engine) runIf(ctx context.Context, p *plan) (Result, error) {
	for _, b := range p.ifBranches {
		ok, err := b.cond(ctx)
		if err != nil {
			return Result{}, err
		}
		if ok {
			return e.runBody(ctx, b.body)
		}
	}
	return e.runBody(ctx, p.ifElse)
}

func (e *Engine) runBody(ctx context.Context, body []*plan) (Result, error) {
	out := Result{Kind: sql.StmtIf}
	for _, sp := range body {
		res, err := e.run(ctx, sp)
		if err != nil {
			return out, err
		}
		out.Nested = append(out.Nested, res)
	}
	return out, nil
}

// exists reports whether a prepared SELECT matches at least one object. Like
// ASSERT it stops at the first match, so an EXISTS in an IF condition is cheap
// even on large collections.
func (e *Engine) exists(ctx context.Context, p *plan) (bool, error) {
	if p.isJoin {
		res, err := e.runJoin(ctx, p)
		if err != nil {
			return false, err
		}
		return len(res.Rows) > 0, nil
	}
	found := false
	err := e.forEachMatch(ctx, p, func(*unstructured.Unstructured) error {
		found = true
		return errStopIteration
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return false, err
	}
	return found, nil
}

// errStopIteration is a sentinel returned from a streaming callback to halt
// listing early (e.g. once an ASSERT has enough evidence to decide). It is not a
// real error and is unwrapped by eachItem so callers can compare against it.
var errStopIteration = errors.New("d8sql: stop iteration")

// runAssert evaluates the inner SELECT and checks it against the expectation,
// returning a *ValidationError when the expectation is not met. For single-table
// inner queries it stops at the first match (a single match already decides
// either EMPTY or NOT EMPTY), keeping the check cheap on large clusters.
func (e *Engine) runAssert(ctx context.Context, p *plan) (Result, error) {
	inner := p.assertInner
	var sample Result
	matched := 0

	if inner.isJoin {
		res, err := e.runJoin(ctx, inner)
		if err != nil {
			return Result{}, err
		}
		matched = len(res.Rows)
		sample = Result{Kind: sql.StmtSelect, Columns: res.Columns}
		if matched > 0 {
			sample.Rows = res.Rows[:1]
		}
	} else {
		sample = Result{Kind: sql.StmtSelect}
		if !inner.star {
			sample.Columns = inner.headers
		}
		err := e.forEachMatch(ctx, inner, func(u *unstructured.Unstructured) error {
			matched++
			if inner.star {
				sample.Objects = append(sample.Objects, u)
			} else {
				r := row{obj: u.Object}
				cells := make([]any, len(inner.getters))
				for j, g := range inner.getters {
					if v, ok := g(&r); ok {
						cells[j] = v
					}
				}
				sample.Rows = append(sample.Rows, cells)
			}
			return errStopIteration // one match decides the assertion
		})
		if err != nil && !errors.Is(err, errStopIteration) {
			return Result{}, err
		}
	}

	pass := (p.expect == sql.ExpectEmpty && matched == 0) ||
		(p.expect == sql.ExpectNotEmpty && matched > 0)
	if pass {
		return Result{Kind: sql.StmtAssert, Affected: matched}, nil
	}
	return Result{}, &ValidationError{
		Code:     p.failCode,
		Message:  p.failMessage,
		Expect:   p.expect,
		Resource: inner.st.Table.Resource,
		Matched:  matched,
		Sample:   sample,
	}
}

// forEachMatch streams the objects matching a single-table plan and invokes fn
// for each one. It pushes namespace/name down to the API server and applies the
// compiled WHERE predicate client-side. Listing is paginated (continue token),
// so only one page is held in memory at a time — fn is called as objects arrive.
func (e *Engine) forEachMatch(ctx context.Context, p *plan, fn func(*unstructured.Unstructured) error) error {
	// A virtual table is already in memory: no listing, no pushdown, just the
	// compiled predicate over the registered rows.
	if p.isVirtual {
		for _, r := range p.virtualRows {
			rw := row{obj: r}
			if p.pred != nil && !p.pred(&rw) {
				continue
			}
			if err := fn(&unstructured.Unstructured{Object: r}); err != nil {
				return err
			}
		}
		return nil
	}

	// The GET fast path needs a single concrete namespace for namespaced
	// resources; otherwise fall back to LIST + client-side filter.
	if p.hasName && (!p.res.namespaced || len(p.namespaces) == 1) {
		ns := ""
		if p.res.namespaced {
			ns = p.namespaces[0]
		}
		obj, err := e.ri(p.res, ns).Get(ctx, p.getName, metav1.GetOptions{})
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return errf("get %s/%s: %v", ns, p.getName, err)
		}
		if p.pred == nil || p.pred(&row{obj: obj.Object}) {
			return fn(obj)
		}
		return nil
	}

	emit := func(u *unstructured.Unstructured) error {
		if p.pred == nil || p.pred(&row{obj: u.Object}) {
			return fn(u)
		}
		return nil
	}

	// nil/empty namespace set (or cluster-scoped) => one cluster-wide stream;
	// a single namespace => one scoped stream; several (namespace IN (...)) =>
	// one List per namespace, run with bounded concurrency.
	switch {
	case !p.res.namespaced || len(p.namespaces) == 0:
		return e.eachItem(ctx, p.res, "", p.labelSel, "", emit)
	case len(p.namespaces) == 1:
		return e.eachItem(ctx, p.res, p.namespaces[0], p.labelSel, "", emit)
	default:
		tasks := make([]listTask, len(p.namespaces))
		for i, ns := range p.namespaces {
			tasks[i] = listTask{res: p.res, ns: ns, labelSel: p.labelSel}
		}
		return e.streamParallel(ctx, tasks, emit)
	}
}

func (e *Engine) runSelect(ctx context.Context, p *plan) (Result, error) {
	out := Result{Kind: sql.StmtSelect}
	if !p.star {
		out.Columns = p.headers
	}
	err := e.forEachMatch(ctx, p, func(u *unstructured.Unstructured) error {
		if p.star {
			out.Objects = append(out.Objects, u)
			return nil
		}
		r := row{obj: u.Object}
		cells := make([]any, len(p.getters))
		for j, g := range p.getters {
			if v, ok := g(&r); ok {
				cells[j] = v
			}
		}
		out.Rows = append(out.Rows, cells)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return out, nil
}

func (e *Engine) runUpdate(ctx context.Context, p *plan) (Result, error) {
	out := Result{Kind: sql.StmtUpdate}
	err := e.forEachMatch(ctx, p, func(item *unstructured.Unstructured) error {
		updated, changed, err := e.patchOne(ctx, p, item)
		if err != nil {
			return err
		}
		if changed {
			out.Objects = append(out.Objects, updated)
			out.Affected++
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// patchOne applies the SET clause to one object as a JSON merge patch that
// touches only the assigned fields, guarded by the object's resourceVersion
// (optimistic concurrency). A concurrent change therefore cannot be silently
// clobbered: on a conflict it re-reads the object, re-checks the WHERE
// predicate, and retries with the fresh version. changed is false when the
// object stopped matching WHERE or disappeared in the meantime.
func (e *Engine) patchOne(ctx context.Context, p *plan, item *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	ri := e.ri(p.res, item.GetNamespace())
	name := item.GetName()
	cur := item
	var (
		result  *unstructured.Unstructured
		changed bool
	)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		data, err := mergePatchData(cur.GetResourceVersion(), p.st.Assignments)
		if err != nil {
			return err
		}
		updated, perr := ri.Patch(ctx, name, types.MergePatchType, data, metav1.PatchOptions{})
		if perr == nil {
			result, changed = updated, true
			return nil
		}
		if !apierrors.IsConflict(perr) {
			return perr
		}
		// The object changed since we read it. Re-read and re-check WHERE so the
		// UPDATE ... WHERE predicate keeps holding for what we actually patch.
		fresh, gerr := ri.Get(ctx, name, metav1.GetOptions{})
		if gerr != nil {
			if isNotFound(gerr) {
				return nil // gone: nothing to update
			}
			return gerr
		}
		if p.pred != nil && !p.pred(&row{obj: fresh.Object}) {
			return nil // no longer matches WHERE: skip
		}
		cur = fresh
		return perr // still matches: retry the patch with the fresh resourceVersion
	})
	if err != nil {
		return nil, false, errf("update %s/%s: %v", item.GetNamespace(), name, err)
	}
	return result, changed, nil
}

func (e *Engine) runDelete(ctx context.Context, p *plan) (Result, error) {
	out := Result{Kind: sql.StmtDelete}
	err := e.forEachMatch(ctx, p, func(item *unstructured.Unstructured) error {
		deleted, err := e.deleteOne(ctx, p, item)
		if err != nil {
			return err
		}
		if deleted {
			out.Affected++
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// deleteOne deletes one object guarded by resourceVersion + UID preconditions,
// so it never removes an object that was modified, or deleted and recreated
// under the same name (a different UID), since it was read. On a conflicting
// change it re-reads, re-checks WHERE, and retries; deleted is false when the
// object stopped matching WHERE or was already gone.
func (e *Engine) deleteOne(ctx context.Context, p *plan, item *unstructured.Unstructured) (bool, error) {
	ri := e.ri(p.res, item.GetNamespace())
	name := item.GetName()
	cur := item
	deleted := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		derr := ri.Delete(ctx, name, deletePreconditions(cur))
		if derr == nil {
			deleted = true
			return nil
		}
		if isNotFound(derr) {
			return nil // already gone
		}
		if !apierrors.IsConflict(derr) {
			return derr
		}
		fresh, gerr := ri.Get(ctx, name, metav1.GetOptions{})
		if gerr != nil {
			if isNotFound(gerr) {
				return nil
			}
			return gerr
		}
		if p.pred != nil && !p.pred(&row{obj: fresh.Object}) {
			return nil // no longer matches WHERE: skip
		}
		cur = fresh
		return derr // retry with the fresh preconditions
	})
	if err != nil {
		return false, errf("delete %s/%s: %v", item.GetNamespace(), name, err)
	}
	return deleted, nil
}

// deletePreconditions builds delete options that pin the object's
// resourceVersion and UID, when known, for optimistic concurrency.
func deletePreconditions(obj *unstructured.Unstructured) metav1.DeleteOptions {
	pc := &metav1.Preconditions{}
	if rv := obj.GetResourceVersion(); rv != "" {
		pc.ResourceVersion = &rv
	}
	if uid := obj.GetUID(); uid != "" {
		pc.UID = &uid
	}
	return metav1.DeleteOptions{Preconditions: pc}
}

// runJoin performs an equi-join (hash join) of two resources with two pushdown
// optimizations:
//
//  1. Label-selector pushdown: a side constrained by metadata.labels.* in WHERE
//     is filtered server-side via labelSelector.
//  2. Field-selector pushdown: the label-filtered side is hashed first (it is
//     usually small), then the other side is fetched with one List per distinct
//     join-key value using a fieldSelector, instead of listing the whole
//     collection. If the field is not server-selectable, it falls back to a full
//     List + client-side join.
//
// The compiled WHERE predicate is always applied client-side, so the pushdowns
// can only reduce work, never change results.
func (e *Engine) runJoin(ctx context.Context, p *plan) (Result, error) {
	// Hash (build) the side that is label-filtered; probe the other side.
	buildRight := !(p.leftLabelSel != "" && p.rightLabelSel == "")

	// Joins always span all namespaces (ns empty); WHERE narrows results.
	build := joinSide{res: p.res, ns: NamespaceAll, labelSel: p.leftLabelSel, key: p.leftKey, keyPath: p.leftKeyPath, isRight: false}
	probe := joinSide{res: p.rightRes, ns: NamespaceAll, labelSel: p.rightLabelSel, key: p.rightKey, keyPath: p.rightKeyPath, isRight: true}
	if buildRight {
		build, probe = probe, build
		build.isRight, probe.isRight = true, false
	}

	buildItems, err := e.collect(ctx, build.res, build.ns, build.labelSel, "")
	if err != nil {
		return Result{}, err
	}

	index := make(map[any][]int, len(buildItems))
	keys := make([]string, 0, len(buildItems))
	seenKey := make(map[string]struct{}, len(buildItems))
	stringKeys := true
	for i := range buildItems {
		v, ok := build.key(rowFor(build.isRight, buildItems[i].Object))
		if !ok {
			continue
		}
		index[normalizeKey(v)] = append(index[normalizeKey(v)], i)
		if s, ok := v.(string); ok && fieldSelectorSafe(s) {
			if _, dup := seenKey[s]; !dup {
				seenKey[s] = struct{}{}
				keys = append(keys, s)
			}
		} else {
			stringKeys = false
		}
	}

	out := Result{Kind: sql.StmtSelect, Columns: p.headers}
	// Stream the probe side and join each item against the in-memory build index.
	err = e.forEachProbe(ctx, probe, keys, stringKeys, func(u *unstructured.Unstructured) error {
		v, ok := probe.key(rowFor(probe.isRight, u.Object))
		if !ok {
			return nil
		}
		for _, bi := range index[normalizeKey(v)] {
			var r row
			if probe.isRight {
				r = row{left: buildItems[bi].Object, right: u.Object}
			} else {
				r = row{left: u.Object, right: buildItems[bi].Object}
			}
			if p.pred != nil && !p.pred(&r) {
				continue
			}
			cells := make([]any, len(p.getters))
			for j, g := range p.getters {
				if val, ok := g(&r); ok {
					cells[j] = val
				}
			}
			out.Rows = append(out.Rows, cells)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return out, nil
}

// joinSide bundles everything needed to list and key one side of a join.
type joinSide struct {
	res      resolved
	ns       string
	labelSel string
	key      getter
	keyPath  []string
	isRight  bool
}

// rowFor builds a single-sided row for key extraction.
func rowFor(isRight bool, obj map[string]any) *row {
	if isRight {
		return &row{right: obj}
	}
	return &row{left: obj}
}

// forEachProbe streams the probe side of a join, preferring a field-selector
// pushdown over the distinct build-side keys and falling back to a full,
// paginated List when the field is not server-selectable or the key set is
// unsuitable. Items are de-duplicated by namespace/name across per-key lists.
func (e *Engine) forEachProbe(ctx context.Context, probe joinSide, keys []string, stringKeys bool, fn func(*unstructured.Unstructured) error) error {
	if stringKeys && len(keys) > 0 && len(keys) <= e.maxFieldSelectorKeys && len(probe.keyPath) > 0 {
		fieldKey := dotPath(probe.keyPath)
		seen := make(map[string]struct{})
		dedup := func(u *unstructured.Unstructured) error {
			id := u.GetNamespace() + "/" + u.GetName()
			if _, dup := seen[id]; dup {
				return nil
			}
			seen[id] = struct{}{}
			return fn(u)
		}
		// An unsupported field selector fails on the first List (before any item
		// is emitted), so we can safely fall back to a full List in that case.
		// The first key is probed sequentially to detect support; the remaining
		// keys are listed with bounded concurrency. streamParallel serializes
		// dedup, so the shared `seen` map needs no extra locking.
		err := e.eachItem(ctx, probe.res, probe.ns, probe.labelSel, fieldKey+"="+keys[0], dedup)
		if err == nil {
			if len(keys) == 1 {
				return nil
			}
			tasks := make([]listTask, 0, len(keys)-1)
			for _, v := range keys[1:] {
				tasks = append(tasks, listTask{res: probe.res, ns: probe.ns, labelSel: probe.labelSel, fieldSel: fieldKey + "=" + v})
			}
			return e.streamParallel(ctx, tasks, dedup)
		}
		// fall through to a full List
	}
	return e.eachItem(ctx, probe.res, probe.ns, probe.labelSel, "", fn)
}

// eachItem streams every object of res in ns (optionally filtered server-side by
// labelSel/fieldSel) via a continue-token pager, invoking fn per item. The pager
// chunks by e.pageSize and transparently falls back to a full List if the
// continue token expires. fn sees one item at a time; only a single page is held
// in memory.
func (e *Engine) eachItem(ctx context.Context, res resolved, ns, labelSel, fieldSel string, fn func(*unstructured.Unstructured) error) error {
	pg := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		if labelSel != "" {
			opts.LabelSelector = labelSel
		}
		if fieldSel != "" {
			opts.FieldSelector = fieldSel
		}
		return e.ri(res, ns).List(ctx, opts)
	}))
	if e.pageSize > 0 {
		pg.PageSize = e.pageSize
	} else {
		pg.PageSize = 0 // unbounded single List
	}
	err := pg.EachListItem(ctx, metav1.ListOptions{}, func(o runtime.Object) error {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			return nil
		}
		return fn(u)
	})
	if err != nil {
		// A deliberate early stop is not a real error; pass it through unwrapped
		// so callers can detect it.
		if errors.Is(err, errStopIteration) {
			return err
		}
		where := res.gvr.Resource
		if ns != "" {
			where += " in " + ns
		}
		return errf("list %s: %v", where, err)
	}
	return nil
}

// listTask describes one List stream: a resource scoped to a namespace with
// optional server-side label/field selectors.
type listTask struct {
	res      resolved
	ns       string
	labelSel string
	fieldSel string
}

// streamParallel runs the given list tasks, bounded by e.listConcurrency, and
// delivers every item to fn. Items are serialized through a mutex, so fn need
// not be concurrency-safe and shared result state stays consistent; only the
// network List round-trips overlap. The first error cancels the remaining work.
func (e *Engine) streamParallel(ctx context.Context, tasks []listTask, fn func(*unstructured.Unstructured) error) error {
	limit := e.listConcurrency
	if limit < 1 {
		limit = 1
	}
	if limit == 1 || len(tasks) <= 1 {
		for _, t := range tasks {
			if err := e.eachItem(ctx, t.res, t.ns, t.labelSel, t.fieldSel, fn); err != nil {
				return err
			}
		}
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex // serializes both fn and access to firstErr
		firstErr error
		errOnce  sync.Once
	)
	fail := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, t := range tasks {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(t listTask) {
			defer wg.Done()
			defer func() { <-sem }()
			err := e.eachItem(ctx, t.res, t.ns, t.labelSel, t.fieldSel, func(u *unstructured.Unstructured) error {
				mu.Lock()
				defer mu.Unlock()
				return fn(u)
			})
			if err != nil {
				fail(err)
			}
		}(t)
	}
	wg.Wait()
	return firstErr
}

// collect streams res into a slice (used where the full set is needed, e.g. the
// build side of a join). It copies the item structs so the per-page backing
// arrays can be released; only the object maps (the actual data) are retained.
func (e *Engine) collect(ctx context.Context, res resolved, ns, labelSel, fieldSel string) ([]unstructured.Unstructured, error) {
	var out []unstructured.Unstructured
	err := e.eachItem(ctx, res, ns, labelSel, fieldSel, func(u *unstructured.Unstructured) error {
		out = append(out, *u)
		return nil
	})
	return out, err
}

// fieldSelectorSafe reports whether v is safe to embed verbatim in a field
// selector (no separators that would corrupt the selector string).
func fieldSelectorSafe(v string) bool {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case ',', '=', '\n':
			return false
		}
	}
	return v != ""
}

// dotPath joins path segments with '.', e.g. ["spec","nodeName"] -> "spec.nodeName".
func dotPath(p []string) string {
	switch len(p) {
	case 0:
		return ""
	case 1:
		return p[0]
	}
	n := len(p) - 1
	for _, s := range p {
		n += len(s)
	}
	b := make([]byte, 0, n)
	for i, s := range p {
		if i > 0 {
			b = append(b, '.')
		}
		b = append(b, s...)
	}
	return string(b)
}

// labelSelectorFor builds a Kubernetes label selector from the conjunctive
// (AND-only) metadata.labels.* constraints in where. A field qualifies when its
// table is alias or (when acceptUnqualified) it is unqualified.
//
// It covers equality (=, !=), set membership (IN, NOT IN) and existence
// (IS [NOT] NULL). Because the compiled WHERE predicate re-checks every object
// client-side, the selector only needs to be a *superset* of the matching set:
// it can prune how many objects are fetched but can never drop a valid result.
// Requirements that fail Kubernetes validation are skipped (left to the
// client-side filter) rather than producing a malformed selector.
func labelSelectorFor(where sql.Expr, alias string, acceptUnqualified bool) string {
	var reqs []labels.Requirement
	collectLabelReqs(where, alias, acceptUnqualified, &reqs)
	if len(reqs) == 0 {
		return ""
	}
	return labels.NewSelector().Add(reqs...).String()
}

func collectLabelReqs(e sql.Expr, alias string, acceptUnqualified bool, reqs *[]labels.Requirement) {
	switch n := e.(type) {
	case *sql.Logic:
		if n.Op != sql.AND {
			return
		}
		collectLabelReqs(n.Left, alias, acceptUnqualified, reqs)
		collectLabelReqs(n.Right, alias, acceptUnqualified, reqs)
	case *sql.Compare:
		f, lit := labelOperands(n)
		if f == nil {
			return
		}
		key, ok := labelKey(f, alias, acceptUnqualified)
		if !ok {
			return
		}
		switch n.Op {
		case sql.EQ:
			addLabelReq(reqs, key, selection.Equals, lit.Str)
		case sql.NEQ:
			addLabelReq(reqs, key, selection.NotEquals, lit.Str)
		}
	case *sql.In:
		f, ok := n.Field.(*sql.Field)
		if !ok {
			return
		}
		key, ok := labelKey(f, alias, acceptUnqualified)
		if !ok {
			return
		}
		vals := make([]string, 0, len(n.Values))
		for _, l := range n.Values {
			if l.Kind != sql.LitString {
				return // mixed/non-string set: leave to client-side filter
			}
			vals = append(vals, l.Str)
		}
		if len(vals) == 0 {
			return
		}
		op := selection.In
		if n.Negate {
			op = selection.NotIn
		}
		addLabelReq(reqs, key, op, vals...)
	case *sql.IsNull:
		f, ok := n.Field.(*sql.Field)
		if !ok {
			return
		}
		key, ok := labelKey(f, alias, acceptUnqualified)
		if !ok {
			return
		}
		if n.Negate { // IS NOT NULL -> label must exist
			addLabelReq(reqs, key, selection.Exists)
		} else { // IS NULL -> label must not exist
			addLabelReq(reqs, key, selection.DoesNotExist)
		}
	}
}

// addLabelReq appends a validated requirement; invalid keys/values are silently
// dropped so the malformed constraint falls back to client-side filtering.
func addLabelReq(reqs *[]labels.Requirement, key string, op selection.Operator, vals ...string) {
	r, err := labels.NewRequirement(key, op, vals)
	if err != nil {
		return
	}
	*reqs = append(*reqs, *r)
}

// labelKey returns the label key when f is metadata.labels.'<key>' on the
// accepted table.
func labelKey(f *sql.Field, alias string, acceptUnqualified bool) (string, bool) {
	if f.Table != alias && !(acceptUnqualified && f.Table == "") {
		return "", false
	}
	if len(f.Path) != 3 || f.Path[0] != "metadata" || f.Path[1] != "labels" {
		return "", false
	}
	return f.Path[2], true
}

// labelOperands returns the field and string-literal operands of a comparison in
// either order, or (nil, nil) if it is not field <op> string-literal.
func labelOperands(c *sql.Compare) (*sql.Field, *sql.Lit) {
	if f, ok := c.Left.(*sql.Field); ok {
		if l, ok := c.Right.(*sql.Lit); ok && l.Kind == sql.LitString {
			return f, l
		}
	}
	if f, ok := c.Right.(*sql.Field); ok {
		if l, ok := c.Left.(*sql.Lit); ok && l.Kind == sql.LitString {
			return f, l
		}
	}
	return nil, nil
}

// projection compiles the SELECT list into getters plus headers.
func (c *compiler) projection(st *sql.Statement) ([]getter, []string) {
	getters := make([]getter, len(st.Columns))
	headers := make([]string, len(st.Columns))
	for i, col := range st.Columns {
		getters[i] = c.compileField(col.Field)
		headers[i] = columnHeader(col)
	}
	return getters, headers
}

func columnHeader(col sql.ColumnRef) string {
	if col.Alias != "" {
		return col.Alias
	}
	return col.Field.String()
}

// mergePatchData builds a JSON merge patch (RFC 7386) from the SET clause,
// containing only the assigned fields. A NULL assignment becomes a JSON null,
// which merge-patch interprets as "remove this key" (matching the previous
// RemoveNestedField behavior). When resourceVersion is non-empty it is embedded
// so the API server enforces optimistic concurrency on the patch.
func mergePatchData(resourceVersion string, as []sql.Assignment) ([]byte, error) {
	root := map[string]any{}
	for _, a := range as {
		setNestedJSON(root, a.Field.Path, litValue(a.Value))
	}
	if resourceVersion != "" {
		setNestedJSON(root, []string{"metadata", "resourceVersion"}, resourceVersion)
	}
	data, err := json.Marshal(root)
	if err != nil {
		return nil, errf("build patch: %v", err)
	}
	return data, nil
}

// setNestedJSON sets val at the given path inside root, creating intermediate
// maps and merging into any that already exist.
func setNestedJSON(root map[string]any, path []string, val any) {
	m := root
	for i := 0; i < len(path)-1; i++ {
		child, ok := m[path[i]].(map[string]any)
		if !ok {
			child = map[string]any{}
			m[path[i]] = child
		}
		m = child
	}
	m[path[len(path)-1]] = val
}

// normalizeKey makes numeric join keys comparable regardless of int/float origin.
func normalizeKey(v any) any {
	if f, ok := toFloat(v); ok {
		return f
	}
	return v
}

// extractNamespaces pulls a conjunctive metadata.namespace constraint out of the
// WHERE clause so it can be pushed down server-side. It supports both
// `metadata.namespace = 'x'` (one namespace) and `metadata.namespace IN (...)`
// (a set, listed per namespace). It only descends through AND nodes to stay
// correct under disjunction; NOT IN is not pushed down.
func extractNamespaces(e sql.Expr) ([]string, bool) {
	switch n := e.(type) {
	case *sql.Logic:
		if n.Op != sql.AND {
			return nil, false
		}
		if v, ok := extractNamespaces(n.Left); ok {
			return v, true
		}
		return extractNamespaces(n.Right)
	case *sql.Compare:
		if n.Op == sql.EQ {
			if v, ok := extractStringField(n, "namespace"); ok {
				return []string{v}, true
			}
		}
	case *sql.In:
		if n.Negate {
			return nil, false
		}
		f, ok := n.Field.(*sql.Field)
		if !ok || !isMetadataField(f, "namespace") {
			return nil, false
		}
		out := make([]string, 0, len(n.Values))
		seen := make(map[string]struct{}, len(n.Values))
		for _, l := range n.Values {
			if l.Kind != sql.LitString {
				return nil, false
			}
			if _, dup := seen[l.Str]; dup {
				continue
			}
			seen[l.Str] = struct{}{}
			out = append(out, l.Str)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

// extractName pulls a conjunctive metadata.name = '...' constraint for the GET
// fast path.
func extractName(e sql.Expr) (string, bool) {
	return extractStringField(e, "name")
}

func extractStringField(e sql.Expr, key string) (string, bool) {
	switch n := e.(type) {
	case *sql.Logic:
		if n.Op != sql.AND {
			return "", false
		}
		if v, ok := extractStringField(n.Left, key); ok {
			return v, true
		}
		return extractStringField(n.Right, key)
	case *sql.Compare:
		if n.Op != sql.EQ {
			return "", false
		}
		if f, ok := n.Left.(*sql.Field); ok && isMetadataField(f, key) {
			if l, ok := n.Right.(*sql.Lit); ok && l.Kind == sql.LitString {
				return l.Str, true
			}
		}
		if f, ok := n.Right.(*sql.Field); ok && isMetadataField(f, key) {
			if l, ok := n.Left.(*sql.Lit); ok && l.Kind == sql.LitString {
				return l.Str, true
			}
		}
	}
	return "", false
}

func isMetadataField(f *sql.Field, key string) bool {
	return f.Table == "" && len(f.Path) == 2 && f.Path[0] == "metadata" && f.Path[1] == key
}

func isNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}
