# d8sql

`d8sql` is a fast, low-allocation Go library that runs **SQL-like statements against Kubernetes resources**. It ships with its own lexer/parser/AST (no SQL engine dependency), resolves resource names through a cached REST mapper, and executes operations through the dynamic client.

It also provides a CLI (`d8sql`) that uses your standard kubeconfig.

## Why it is fast

- **Pull-based lexer** — no goroutines/channels (unlike `text/template/parse`). Tokens alias the source string, so scanning allocates nothing.
- **Allocation-free keyword lookup** — identifiers are upper-cased into a stack buffer; the map lookup avoids heap allocation.
- **Compiled predicates** — the `WHERE` clause is compiled once into closures. Per-object evaluation does **0 allocations** and walks the unstructured object directly (no reflection, no map dispatch).
- **Pushdown** — constraints are pushed to the API server instead of being filtered client-side:
  - `metadata.namespace = 'x'` lists one namespace, `metadata.namespace IN (...)` issues one scoped `List` per namespace (instead of an all-cluster scan), and a `metadata.name` equality turns a `LIST` into a `GET`.
  - `metadata.labels.*` constraints become a server-side `labelSelector`, covering equality (`=`, `!=`), set membership (`IN`, `NOT IN`) and existence (`IS [NOT] NULL`) — e.g. `WHERE metadata.labels.'app' IN ('redis','memcached')` is sent as `app in (memcached,redis)`. This is the big lever on large clusters: the API server (and `etcd`) return only the matching objects rather than the whole collection. Selectors are built and validated with the official `k8s.io/apimachinery/pkg/labels` package; only `AND`-connected constraints are pushed, and the compiled `WHERE` predicate still re-checks every object, so a pushdown can only narrow the fetch, never change the result.
- **Hash join** for the JOIN case instead of a nested loop, with two pushdowns:
  - *label-selector pushdown* — a side constrained by `metadata.labels.*` is filtered server-side via `labelSelector`;
  - *field-selector pushdown* — the label-filtered (smaller) side is hashed first, then the other side is fetched with one `List` per distinct join-key value using a `fieldSelector` (e.g. `spec.nodeName=<node>`), instead of listing the whole collection. Falls back to a full `List` if the field is not server-selectable or when there are more than `WithMaxFieldSelectorKeys` (default 128) distinct keys.
- **Chunked, streaming `List`** — listing is paginated with a `continue` token (`WithPageSize`, default 500) and objects are processed page by page as they arrive. Only one page is held in memory at a time, so `SELECT`/`UPDATE`/`DELETE` over huge collections (e.g. secrets) stay memory-bounded instead of materializing the whole list. The pager transparently falls back to a full `List` if the continue token expires.
- **Bounded-concurrency fan-out** — when a query expands into several independent lists (`metadata.namespace IN (...)` → one `List` per namespace; the join field-selector probe → one `List` per key value), they run concurrently up to `WithListConcurrency` (default 8) to overlap network round-trips. Items are still delivered one at a time, so result handling stays deterministic and writes (`UPDATE`/`DELETE`) are never issued concurrently — keeping API-server load in check.
- **Per-instance caching** of resource resolution (GVR + scope) and discovery.

### Benchmarks (Apple M4 Pro, Go 1.26)

Evaluating the same predicate over the same object, `d8sql` vs [cel-go](https://github.com/google/cel-go):

| Predicate | d8sql | CEL | Speedup |
|---|---|---|---|
| `status.phase = 'Running'` | **18.76 ns/op, 0 allocs** | 63.18 ns/op, 1 alloc | ~3.4× |
| `phase = 'Running' AND replicas >= 1 AND name LIKE '%redis%'` | **62.03 ns/op, 0 allocs** | 198.7 ns/op, 3 allocs | ~3.2× |

Parse + compile of the compound query: 703.4 ns/op, 984 B/op, 22 allocs/op (a one-time cost per statement).

Raw results (go1.26.4, darwin/arm64, Apple M4 Pro):

```text
BenchmarkD8SQL_Equality-12        65734611    18.76 ns/op      0 B/op    0 allocs/op
BenchmarkCEL_Equality-12          17896101    63.18 ns/op     16 B/op    1 allocs/op
BenchmarkD8SQL_Compound-12        19020382    62.03 ns/op      0 B/op    0 allocs/op
BenchmarkCEL_Compound-12           6047913   198.7 ns/op      48 B/op    3 allocs/op
BenchmarkD8SQL_ParseCompile-12     1677906   703.4 ns/op     984 B/op   22 allocs/op
```

The benchmark harness itself was removed from the repository: it was the only
thing pulling in `cel-go`, and keeping the library dependency-light for
consumers matters more than a re-runnable comparison.

## Install

```bash
go get github.com/deckhouse/d8sql          # library
go install github.com/deckhouse/d8sql/cmd/d8sql@latest  # CLI
```

## CLI usage

```bash
d8sql "SQL STATEMENT; [SQL STATEMENT; ...]"
```

Flags:

| Flag | Description | Default |
|---|---|---|
| `-n`, `--namespace` | scope queries to a namespace | all namespaces |
| `-o`, `--output` | output format: `table`, `yaml`, `json` | `table` |
| `--kubeconfig` | path to kubeconfig | `$KUBECONFIG` or `~/.kube/config` |

```bash
d8sql "SELECT metadata.name, status.phase FROM pods WHERE status.phase = 'Running'"
d8sql -n kube-system -o yaml "SELECT * FROM pods"
```

## Library usage

```go
cfg, _ := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
    clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()

// Build once; resolution/discovery is cached on the instance.
// Queries span all namespaces by default; use WithDefaultNamespace to scope.
engine, _ := d8sql.NewForConfig(cfg)

res, err := engine.ExecuteOne(ctx,
    "SELECT metadata.name, status.phase FROM pods WHERE status.phase = 'Running'")
for _, r := range res.Rows {
    fmt.Println(r) // []any: [name, phase]
}
```

You can also inject your own already-cached dynamic client and REST mapper:

```go
engine := d8sql.New(dynClient, restMapper)
```

Options:

- `d8sql.WithDefaultNamespace(ns)` — scope queries that omit `metadata.namespace` to `ns` (default: all namespaces).
- `d8sql.WithMaxFieldSelectorKeys(n)` — cap on per-key field-selector `List` calls in a join before falling back to a full `List` (default 128; `<= 0` disables the pushdown).
- `d8sql.WithPageSize(n)` — `List` chunk size for continue-token pagination (default 500). Objects are streamed and processed one page at a time, bounding memory for large collections. `<= 0` disables chunking (a single, unbounded `List`).
- `d8sql.WithVirtualTable(name, rows)` — register a read-only in-memory table queryable by `name` in `FROM` (see [Virtual tables](#virtual-tables)). May be called several times, once per table.
- `d8sql.WithListConcurrency(n)` — max concurrent `List` calls when a query fans out into several lists, i.e. `metadata.namespace IN (...)` and the join field-selector probe (default 8; `<= 1` lists sequentially). Writes are never concurrent regardless of this setting.

A runnable example lives in [`example/`](./example).

## Resource name formats

All of the following are accepted and resolved through the REST mapper:

1. **Plural**: `pods`, `deployments`, `services` (all standard resources)
2. **Singular**: `pod`, `deployment`, `service`
3. **Group-qualified**: `deployments.apps`, `rolebindings.rbac.authorization.k8s.io`
4. **CRD `<resource>.<group>`**: `moduleconfigs.deckhouse.io`
5. **CRD `<resource>.<version>.<group>`**: `moduleconfigs.v1alpha1.deckhouse.io`

A resource name containing dots can also be single-quoted: `FROM 'envoyproxies.gateway.envoyproxy.io'`.

## Supported SQL

PostgreSQL-flavored syntax. `metadata.namespace` in `WHERE` is always treated as the namespace selector and pushed down server-side — both as an equality (`= 'x'`) and as a set (`IN ('a','b')`, one scoped `List` per namespace). Likewise, `metadata.labels.'<key>'` is always treated as a label and pushed down as a `labelSelector` (equality, `IN`/`NOT IN`, and `IS [NOT] NULL`). When a query does **not** constrain `metadata.namespace`, it spans **all namespaces** by default (use `-n` / `WithDefaultNamespace` to scope). Both `=` and `==` are accepted for equality. Comments (`-- ...` and `/* ... */`) are supported. Transactions are **not** supported. Statements: `SELECT`, `INSERT` (create an object), `UPDATE`, `DELETE`, `ASSERT` (read-only validation) and `IF ... THEN ... END IF` (conditional execution) — all described below.

> Note: because the default is all-namespaces, a mutating statement without a namespace constraint affects the whole cluster (e.g. `DELETE FROM pods` deletes pods in every namespace). Constrain with `WHERE metadata.namespace = '...'` or `-n` to limit scope.

### Concurrency-safe writes

`UPDATE` and `DELETE` use optimistic concurrency so a racing change can never be silently lost:

- **`UPDATE`** is sent as a **JSON merge patch containing only the `SET` fields**, guarded by the object's `resourceVersion`. Concurrent edits to *other* fields are preserved (they are not part of the patch). If the object changed since it was read, the API server returns a conflict; d8sql then re-reads it, **re-checks the `WHERE` predicate**, and retries — so it never patches an object that no longer matches. `SET <field> = NULL` removes the field. (Transactions across multiple objects are still **not** supported.)
- **`INSERT`** is a plain `Create`: the API server's name uniqueness is the guard, so a racing creator makes the statement fail with `AlreadyExists` instead of overwriting an existing object.
- **`DELETE`** is issued with `resourceVersion` + `UID` preconditions, so it never removes an object that was modified, or deleted and recreated under the same name, since it was read. Conflicts are retried with the same re-check of `WHERE`.

### SELECT (get / list)

```sql
-- Basic format: resource name only (spans ALL namespaces by default)
SELECT * FROM pods;

-- Namespace specification
SELECT * FROM pods WHERE metadata.namespace = 'd8-system';

-- Non-namespaced resources (namespace specification is ignored)
SELECT * FROM nodes;

-- List all running pods
SELECT metadata.name, metadata.namespace FROM pods WHERE status.phase = 'Running';

-- List all services like redis
SELECT metadata.name FROM services WHERE metadata.name LIKE '%redis%';

-- Filter by labels (pushed to the API server as a labelSelector)
SELECT metadata.namespace, metadata.name FROM pods
  WHERE metadata.labels.'app' = 'redis';

-- Set-based label match (sent as `app in (memcached,redis)`)
SELECT metadata.name FROM pods
  WHERE metadata.labels.'app' IN ('redis', 'memcached');

-- Existence check (sent as labelSelector `heritage`)
SELECT metadata.name FROM pods WHERE metadata.labels.'heritage' IS NOT NULL;

-- Example for a CRD
SELECT metadata.name FROM 'envoyproxies.gateway.envoyproxy.io';

-- List deployments with ready replicas
SELECT metadata.name, spec.replicas, status.readyReplicas
  FROM deployments WHERE spec.replicas == status.readyReplicas;

-- List all pods with their node names and instance types (JOIN)
SELECT pod.metadata.name, pod.spec.nodeName,
       node.metadata.labels.'node.kubernetes.io/instance-type'
  FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name;

-- All pods running on nodes that belong to the "worker" node group (JOIN + label filter)
SELECT pod.metadata.namespace, pod.metadata.name, pod.spec.nodeName
  FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name
  WHERE node.metadata.labels.'node.deckhouse.io/group' = 'worker';
```

### INSERT (create)

`INSERT` creates exactly one object. The object is described with the same
`SET <field> = <literal>` clause as `UPDATE`, and dotted paths build the nested
document:

```sql
-- Create the configmap `default/foobar`
INSERT INTO configmaps SET
  metadata.name = 'foobar',
  metadata.namespace = 'default',
  data.greeting = 'hello';

-- Labels and nested fields work exactly as in UPDATE
INSERT INTO configmaps SET
  metadata.name = 'settings',
  metadata.namespace = 'd8-system',
  metadata.labels.'heritage' = 'deckhouse',
  data.'log-level' = 'info';
```

**Why `SET` and not `(cols) VALUES (...)`.** In PostgreSQL an `INSERT` is
written as `INSERT INTO t (a, b) VALUES (1, 2)`; the `SET` spelling used here is
a deliberate deviation. A Kubernetes object is a nested document, not a flat row,
and the rest of this dialect already addresses it through dotted field paths
(`data.greeting`, `metadata.labels.'app'`). Reusing the `UPDATE ... SET` shape
keeps one way of writing a field path instead of two, and avoids a positional
column list that would be unreadable for deep paths. The column-list/`VALUES`
form is **not** accepted.

Rules:

- **`metadata.name` is required.** There is no `generateName` support; a missing
  or non-string name is a prepare-time error.
- **Namespace.** For a namespaced resource the namespace is taken from
  `metadata.namespace` in the `SET` clause; if absent, the engine's default
  namespace (`-n` / `WithDefaultNamespace`) is used. If neither is set, the
  statement is rejected — it is never silently created in `default`. For a
  cluster-scoped resource, assigning `metadata.namespace` is an error.
- **`apiVersion`/`kind`** are stamped from the resolved resource, so they never
  have to be written out (and assignments to them are overridden).
- **Virtual tables are read-only**, so `INSERT INTO <virtual table>` is rejected
  at prepare time, like `UPDATE`/`DELETE`.
- **`AlreadyExists` is an error**, not a silent success. The result is
  `Affected: 1` and `Objects` holding the created object as returned by the API
  server.

Idempotent creation (the "create if missing" migration idiom) is expressed with
`IF`, which keeps the check and the creation in one statement:

```sql
IF NOT EXISTS (SELECT * FROM configmaps
               WHERE metadata.namespace = 'default' AND metadata.name = 'foobar') THEN
  INSERT INTO configmaps SET
    metadata.name = 'foobar',
    metadata.namespace = 'default',
    data.greeting = 'hello';
END IF;
```

Note this is a check-then-create, not an atomic upsert: a racing creator can
still win between the `SELECT` and the `INSERT`, in which case the `INSERT`
fails with `AlreadyExists` rather than overwriting anything.

### UPDATE

```sql
-- Update a configmap in the `default` namespace named `foobar`
UPDATE configmap SET data.foo = 'bar'
  WHERE metadata.namespace = 'default' AND metadata.name = 'foobar';

-- Update all pods in the `default` namespace and set label foo=bar
UPDATE pod SET metadata.labels.'foo' = 'bar'
  WHERE metadata.namespace = 'default';

-- Scale up deployments in `default` that have replicas == 0
UPDATE deployment SET spec.replicas = 1
  WHERE metadata.namespace = 'default' AND spec.replicas < 1;
```

### DELETE

```sql
-- Delete all pods from the `default` namespace
DELETE FROM pods WHERE metadata.namespace = 'default';

-- Delete all pods from several namespaces at once (set membership)
DELETE FROM pods WHERE metadata.namespace IN ('d8-chrony', 'd8-pod-reloader');
```

The `IN` form deletes every pod whose `metadata.namespace` is one of the listed
values — here, all pods in `d8-chrony` and `d8-pod-reloader`. A
`metadata.namespace IN (...)` set is **pushed down server-side**: the engine
issues one scoped `List` per namespace (`d8-chrony`, then `d8-pod-reloader`)
instead of listing all pods in the cluster, which matters a lot on large
clusters. The compiled `WHERE` predicate is still re-applied client-side, so the
result is identical to a full scan.

### ASSERT

`ASSERT` validates the cluster state without mutating anything: it runs an inner
`SELECT` and checks whether it matched any objects. It returns a typed
`*d8sql.ValidationError` when the expectation is not met, which makes it useful
for pre-upgrade gates, CI checks and admission-style invariants.

```sql
-- Fail if any IngressNginxController is still on version 1.12
ASSERT EMPTY (
  SELECT metadata.name, spec.version
  FROM ingressnginxcontrollers.deckhouse.io
  WHERE spec.version = '1.12'
) FAIL 'NGINX_1_12' 'IngressNginxControllers on 1.12 must be upgraded first';

-- Fail if there are no Running pods in kube-system (something is wrong)
ASSERT NOT EMPTY (
  SELECT metadata.name FROM pods
  WHERE metadata.namespace = 'kube-system' AND status.phase = 'Running'
);

-- Fail if any Failed pods exist anywhere in the cluster
ASSERT EMPTY (SELECT metadata.namespace, metadata.name FROM pods WHERE status.phase = 'Failed')
  FAIL 'FAILED_PODS';
```

Syntax:

```
ASSERT EMPTY     ( <select> ) [ FAIL '<code>' ['<message>'] ]
ASSERT NOT EMPTY ( <select> ) [ FAIL '<code>' ['<message>'] ]
```

- `EMPTY` passes when the inner `SELECT` matches **zero** objects; `NOT EMPTY`
  passes when it matches **at least one**.
- The optional `FAIL '<code>' '<message>'` sets a machine-readable `Code` and a
  human `Message` on the returned `ValidationError`. The `<message>` is optional.
- The inner `SELECT` benefits from all the usual pushdowns (namespace, name,
  labels). For single-table asserts the engine **stops at the first match** — a
  single object already decides either expectation — so an `ASSERT NOT EMPTY`
  short-circuits immediately and an `ASSERT EMPTY` only scans far enough to find
  one offender.

From Go, detect the failure with `errors.As` and inspect the offending sample:

```go
_, err := engine.ExecuteOne(ctx,
    "ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed') FAIL 'FAILED_PODS'")
var ve *d8sql.ValidationError
if errors.As(err, &ve) {
    log.Printf("assertion %s failed: %s (matched=%d)", ve.Code, ve.Error(), ve.Matched)
    // ve.Sample holds a small sample of the offending rows/objects
}
```

On the CLI a failed assertion prints the error and exits non-zero (the offending
`Sample` is available via the library), so it slots straight into shell pipelines:

```bash
d8sql "ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed') FAIL 'FAILED_PODS'" \
  && echo "no failed pods, proceeding"
```

Inside a multi-statement batch, a failed `ASSERT` stops the batch before any
later statement runs — combine it with a `DELETE`/`UPDATE` to guard mutations.

### Virtual tables

A virtual table is a read-only, in-memory table registered on the engine and
queried by name in `FROM`, next to real cluster resources. It is the way to make
facts the caller already knows — platform version, edition, bundle — available to
SQL, so a validation can be written once and evaluated against both the cluster
and its context:

```go
engine := d8sql.New(dynClient, restMapper,
    d8sql.WithVirtualTable("v_d8_platform", []map[string]any{{
        "deckhouseVersion":  "1.76.9",
        "deckhouseEdition":  "EE",
        "deckhouseBundle":   "Default",
        "kubernetesVersion": "1.31",
    }}),
)
```

```sql
-- Fail unless the platform runs an enterprise edition
ASSERT NOT EMPTY (
  SELECT deckhouseEdition FROM v_d8_platform WHERE deckhouseEdition IN ('EE','FE')
) FAIL 'EDITION' 'this module requires EE';

SELECT deckhouseVersion, kubernetesVersion FROM v_d8_platform;
```

- Rows are plain maps addressed with the same dotted paths as cluster objects, so
  nested values work too; the full `WHERE` operator set applies.
- The name is matched exactly as written in `FROM` and takes precedence over a
  cluster resource of the same name. The `v_` prefix is a convention, not a rule.
- Virtual tables are **query-only**: `UPDATE`, `DELETE` and `JOIN` against one are
  rejected at prepare time (so, like any other prepare error, before a batch
  mutates anything).
- No API server call is made, and no namespace/label pushdown applies — the rows
  are already in memory.

### IF / ELSIF / ELSE

`IF` runs statements conditionally, in the PL/pgSQL shape:

```
IF <cond> THEN <statements> [ELSIF <cond> THEN <statements>]... [ELSE <statements>] END IF
```

```sql
IF EXISTS (SELECT * FROM v_d8_platform WHERE deckhouseEdition = 'EE') THEN
    UPDATE moduleconfigs.deckhouse.io SET spec.enabled = true WHERE metadata.name = 'foo';
ELSIF NOT EXISTS (SELECT * FROM nodes WHERE metadata.labels.'node.deckhouse.io/group' = 'worker') THEN
    ASSERT EMPTY (SELECT * FROM pods WHERE status.phase = 'Failed') FAIL 'FAILED_PODS';
ELSE
    DELETE FROM configmaps WHERE metadata.namespace = 'tmp' AND metadata.name = 'old';
END IF;
```

- A condition is a boolean combination (`AND`, `OR`, `NOT`, parentheses) of
  `[NOT] EXISTS ( <select> )` terms. Field comparisons live inside the subquery's
  `WHERE`, which composes with virtual tables as shown above.
- Branch conditions are evaluated in order and only the first one that holds
  runs; `AND`/`OR` short-circuit, so a decided condition issues no further
  queries. Each `EXISTS` stops at the first matching object, like `ASSERT`.
- Bodies hold semicolon-separated `SELECT`/`UPDATE`/`DELETE`/`ASSERT` statements
  and nested `IF`s, and run sequentially.
- **Every branch is compiled up front**, including the ones that end up not being
  taken, so a syntax or resolution error anywhere inside an `IF` fails the batch
  before any statement mutates the cluster.
- A failing `ASSERT` inside the taken branch returns its `*d8sql.ValidationError`
  to the caller and stops the batch, exactly as at the top level.

The statement yields a single `Result` with `Kind == sql.StmtIf`, whose `Nested`
field holds one `Result` per statement that actually ran, in order (empty when no
branch matched and there was no `ELSE`):

```go
res, err := engine.ExecuteOne(ctx, "IF EXISTS (SELECT * FROM pods WHERE status.phase = 'Failed') THEN DELETE FROM pods WHERE status.phase = 'Failed'; END IF")
for _, r := range res.Nested {
    fmt.Println(r.Kind, r.Affected)
}
```

### Operators

- Comparison: `=`, `==`, `!=` / `<>`, `<`, `<=`, `>`, `>=`
- Pattern matching: `LIKE`, `ILIKE` (case-insensitive), with `%` and `_` wildcards; `NOT LIKE`
- Set membership: `IN (...)`, `NOT IN (...)`
- Null checks: `IS NULL`, `IS NOT NULL`
- Boolean: `AND`, `OR`, `NOT`, parentheses
- Field-to-field comparisons (e.g. `spec.replicas == status.readyReplicas`)
- Quoted path segments for keys with dots/slashes: `metadata.labels.'node.kubernetes.io/instance-type'`

## Notes & limitations

- The dynamic client is used uniformly (returns `unstructured`), which keeps the code allocation-light and works identically for built-ins and CRDs.
- `JOIN` supports a single equi-join (`ON a == b`) implemented as an in-memory hash join. Namespaced sides are listed across **all** namespaces (a `WHERE` clause can narrow the result), so cross-cluster joins like "pods on nodes with label X" work regardless of namespace. Label- and field-selector pushdowns (see above) reduce the data fetched; the compiled `WHERE` predicate is always re-applied client-side, so pushdowns never change results.
- No transactions, but a multi-statement batch is **parsed, resolved and compiled in full before any statement executes**. A syntax/resolution/compilation error in a later statement is reported before earlier (possibly mutating) statements run, so the batch is not partially applied due to a malformed statement. Once execution starts, statements run sequentially and stop at the first runtime error.
- For a projected `SELECT`, `Result.Rows` holds typed `[]any` cells; for `SELECT *`, `Result.Objects` holds the full objects.
- **Cache invalidation:** the resource-resolution cache and the discovery/REST-mapper cache live for the lifetime of the `Engine` and are never invalidated. If a new CRD is installed in the cluster *after* the engine was created, it may fail to resolve (neither the discovery cache nor the resolution cache learns about it). Recreate the `Engine` to pick up newly installed resource types.

## Development

```bash
go test ./...                          # unit + engine tests (fake dynamic client)
go test -run NONE -bench . -benchmem . # benchmarks vs CEL
```
