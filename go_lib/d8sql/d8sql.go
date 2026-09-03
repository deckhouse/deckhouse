// Package d8sql executes SQL-like statements against Kubernetes resources.
//
// It parses SELECT/INSERT/UPDATE/DELETE/ASSERT/IF statements with its own allocation-conscious
// lexer and parser (see the sql subpackage) and runs them through a dynamic
// client. Resource references may be plural, singular, group-qualified or
// fully versioned CRD names; resolution is cached per Engine instance.
package d8sql

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"github.com/deckhouse/d8sql/sql"
)

// NamespaceAll is the namespace value meaning "all namespaces" (the empty
// string, matching client-go's metav1.NamespaceAll).
const NamespaceAll = ""

// defaultMaxFieldSelectorKeys is the default cap on per-key field-selector List
// calls in a join before falling back to a single full List.
const defaultMaxFieldSelectorKeys = 128

// defaultPageSize is the default List chunk size. Listing is paginated with a
// continue token so large collections are streamed and processed page by page
// instead of being held in memory all at once.
const defaultPageSize int64 = 500

// defaultListConcurrency is the default number of concurrent List calls used
// when a query fans out into several independent lists (metadata.namespace
// IN (...) and the join field-selector probe). It bounds the load placed on the
// API server while still overlapping network round-trips.
const defaultListConcurrency = 8

// Engine executes SQL statements against a cluster. It is safe for concurrent
// use: the resolution cache is the only mutable state and is a sync.Map.
type Engine struct {
	dyn    dynamic.Interface
	mapper meta.RESTMapper

	// defaultNS is the namespace used for namespaced resources when a query does
	// not constrain metadata.namespace. Empty means "all namespaces".
	defaultNS string

	// maxFieldSelectorKeys caps how many per-key field-selector List calls a join
	// will issue before falling back to a single full List. Values <= 0 disable
	// the field-selector pushdown entirely.
	maxFieldSelectorKeys int

	// pageSize is the List chunk size (continue-token pagination). Values <= 0
	// disable chunking (a single, unbounded List).
	pageSize int64

	// listConcurrency caps the number of concurrent List calls when a query fans
	// out into several lists (namespace IN, join probe). Values <= 1 run them
	// sequentially.
	listConcurrency int

	// virtual holds the read-only in-memory tables registered with
	// WithVirtualTable, keyed by the name used in FROM. It is written only while
	// options are applied in New, and read-only afterwards.
	virtual map[string][]map[string]any

	cache sync.Map // string (resource arg) -> resolved
}

// resolved is the cached outcome of mapping a resource argument to a GVR. The
// GVK is kept alongside it because INSERT has to stamp apiVersion/kind on the
// object it builds.
type resolved struct {
	gvr        schema.GroupVersionResource
	gvk        schema.GroupVersionKind
	namespaced bool
}

// Option configures an Engine.
type Option func(*Engine)

// WithDefaultNamespace scopes queries that do not constrain metadata.namespace
// to ns. The default (empty string / NamespaceAll) queries all namespaces.
func WithDefaultNamespace(ns string) Option {
	return func(e *Engine) { e.defaultNS = ns }
}

// WithMaxFieldSelectorKeys sets the cap on per-key field-selector List calls
// used by the join pushdown (default 128). Above this many distinct join keys, a
// single full List is used instead. A value <= 0 disables the field-selector
// pushdown, always falling back to a full List.
func WithMaxFieldSelectorKeys(n int) Option {
	return func(e *Engine) { e.maxFieldSelectorKeys = n }
}

// WithPageSize sets the List chunk size (default 500). Listing is paginated via
// a continue token and processed page by page, which bounds memory for large
// collections (e.g. secrets). A value <= 0 disables chunking (a single List).
func WithPageSize(n int64) Option {
	return func(e *Engine) { e.pageSize = n }
}

// WithListConcurrency caps how many List calls run concurrently when a query
// fans out into several independent lists — metadata.namespace IN (...) (one
// List per namespace) and the join field-selector probe (one List per key
// value). The default is 8; a value <= 1 lists sequentially. Items are still
// delivered one at a time, so result handling stays deterministic and writes
// (UPDATE/DELETE) are not issued concurrently.
func WithListConcurrency(n int) Option {
	return func(e *Engine) { e.listConcurrency = n }
}

// WithVirtualTable registers a read-only in-memory table that can be queried by
// name in FROM, alongside real cluster resources — typically facts the caller
// wants queryable from SQL:
//
//	d8sql.WithVirtualTable("v_d8_platform", []map[string]any{{
//	    "deckhouseVersion": "1.76.9",
//	    "deckhouseEdition": "EE",
//	    "deckhouseBundle":  "Default",
//	    "kubernetesVersion": "1.31",
//	}})
//
// Rows are plain maps addressed by the same dotted paths as cluster objects, so
// nested values work as well. The rows are retained, not copied, and must not be
// mutated afterwards. The name is matched exactly as written in FROM and takes
// precedence over a cluster resource of the same name; registering the same name
// twice replaces the previous rows.
//
// Virtual tables are query-only: UPDATE, DELETE and JOIN against one are
// rejected at prepare time.
func WithVirtualTable(name string, rows []map[string]any) Option {
	return func(e *Engine) {
		if e.virtual == nil {
			e.virtual = make(map[string][]map[string]any, 1)
		}
		e.virtual[name] = rows
	}
}

// New constructs an Engine from an already-built dynamic client and REST mapper.
// This is the primary library entry point: callers inject their own cached
// kubectl/dynamic client and mapper. Both are retained for the engine lifetime.
func New(dyn dynamic.Interface, mapper meta.RESTMapper, opts ...Option) *Engine {
	e := &Engine{
		dyn:                  dyn,
		mapper:               mapper,
		defaultNS:            NamespaceAll,
		maxFieldSelectorKeys: defaultMaxFieldSelectorKeys,
		pageSize:             defaultPageSize,
		listConcurrency:      defaultListConcurrency,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// NewForConfig builds an Engine from a REST config, creating a dynamic client
// and a discovery-backed REST mapper. The discovery results are cached in
// memory for the lifetime of the Engine, so resource resolution does not hit the
// API server repeatedly.
func NewForConfig(cfg *rest.Config, opts ...Option) (*Engine, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, errf("create dynamic client: %v", err)
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, errf("create discovery client: %v", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	return New(dyn, mapper, opts...), nil
}

// Result holds the outcome of one statement.
type Result struct {
	Kind sql.StmtKind

	// Columns and Rows are populated for projected SELECTs (i.e. not SELECT *).
	Columns []string
	Rows    [][]any

	// Objects holds the matched/affected objects. For SELECT * it is the full
	// result set; for UPDATE it holds the updated objects; for INSERT it holds
	// the single created object as returned by the API server.
	Objects []*unstructured.Unstructured

	// Affected counts mutated objects for INSERT/UPDATE/DELETE.
	Affected int

	// Nested holds one Result per statement executed inside the taken branch of
	// an IF statement, in execution order. It is set only for Kind == StmtIf and
	// is empty when no branch matched (and there was no ELSE).
	Nested []Result
}

// ValidationError is returned when an ASSERT statement's expectation is not met.
// Callers can detect it with errors.As to react programmatically (for example, to
// gate a CI step or a pre-upgrade check) and inspect the offending Sample.
type ValidationError struct {
	Code     string         // machine-readable code from FAIL '<code>' (may be empty)
	Message  string         // human message from FAIL '<code>' '<message>' (may be empty)
	Expect   sql.ExpectKind // the expectation that failed
	Resource string         // primary resource of the inner SELECT
	Matched  int            // matched objects observed (a lower bound when streaming stopped early)
	Sample   Result         // a small sample of the offending objects/rows
}

func (e *ValidationError) Error() string {
	msg := e.Message
	if msg == "" {
		if e.Expect == sql.ExpectNotEmpty {
			msg = fmt.Sprintf("expected at least one %s, found none", e.Resource)
		} else {
			msg = fmt.Sprintf("expected no %s, found a match", e.Resource)
		}
	}
	if e.Code != "" {
		return fmt.Sprintf("d8sql: validation failed [%s]: %s", e.Code, msg)
	}
	return "d8sql: validation failed: " + msg
}

// Execute parses and runs one or more semicolon-separated statements, returning
// one Result per statement, in order.
//
// The whole batch is parsed, resolved and compiled up front; only if every
// statement prepares successfully does execution begin. This means a syntax,
// resolution or compilation error in a later statement is reported before any
// earlier (possibly mutating) statement runs, avoiding partially-applied
// batches. Execution itself still stops at the first runtime error.
func (e *Engine) Execute(ctx context.Context, query string) ([]Result, error) {
	stmts, err := sql.ParseMany(query)
	if err != nil {
		return nil, err
	}

	// Phase 1: prepare (no mutating cluster operations).
	plans := make([]*plan, len(stmts))
	for i, st := range stmts {
		p, err := e.prepare(st)
		if err != nil {
			return nil, err
		}
		plans[i] = p
	}

	// Phase 2: execute.
	results := make([]Result, 0, len(plans))
	for _, p := range plans {
		res, err := e.run(ctx, p)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// ExecuteOne parses, prepares and runs exactly one statement.
func (e *Engine) ExecuteOne(ctx context.Context, query string) (Result, error) {
	st, err := sql.Parse(query)
	if err != nil {
		return Result{}, err
	}
	p, err := e.prepare(st)
	if err != nil {
		return Result{}, err
	}
	return e.run(ctx, p)
}

// resolve maps a resource argument to its GVR and namespace scope, caching the
// result. It accepts plural, singular, group-qualified and versioned CRD forms.
func (e *Engine) resolve(arg string) (resolved, error) {
	if v, ok := e.cache.Load(arg); ok {
		return v.(resolved), nil
	}
	full, gr := schema.ParseResourceArg(arg)
	var (
		gvr schema.GroupVersionResource
		err error
	)
	if full != nil {
		gvr, err = e.mapper.ResourceFor(*full)
	}
	if full == nil || err != nil {
		gvr, err = e.mapper.ResourceFor(gr.WithVersion(""))
	}
	if err != nil {
		return resolved{}, errf("cannot resolve resource %q: %v", arg, err)
	}
	gvk, err := e.mapper.KindFor(gvr)
	if err != nil {
		return resolved{}, errf("cannot determine kind for %q: %v", arg, err)
	}
	mapping, err := e.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return resolved{}, errf("cannot map %q: %v", arg, err)
	}
	res := resolved{gvr: gvr, gvk: gvk, namespaced: mapping.Scope.Name() == meta.RESTScopeNameNamespace}
	e.cache.Store(arg, res)
	return res, nil
}

// ri returns the dynamic resource interface scoped to ns when applicable.
func (e *Engine) ri(res resolved, ns string) dynamic.ResourceInterface {
	r := e.dyn.Resource(res.gvr)
	if res.namespaced && ns != "" {
		return r.Namespace(ns)
	}
	return r
}

func errf(format string, a ...any) error {
	return fmt.Errorf("d8sql: "+format, a...)
}
