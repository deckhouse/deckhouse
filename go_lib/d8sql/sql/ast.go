package sql

// StmtKind enumerates the supported statement types.
type StmtKind uint8

const (
	StmtSelect StmtKind = iota
	StmtUpdate
	StmtDelete
	StmtAssert
	StmtIf
)

// ExpectKind is the result expectation of an ASSERT statement.
type ExpectKind uint8

const (
	ExpectEmpty    ExpectKind = iota // ASSERT EMPTY (...): zero matching rows expected
	ExpectNotEmpty                   // ASSERT NOT EMPTY (...): at least one row expected
)

// Statement is a single parsed SQL statement.
type Statement struct {
	Kind StmtKind

	Table TableRef    // primary FROM target
	Join  *JoinClause // optional, SELECT only

	Star    bool        // SELECT *
	Columns []ColumnRef // projection list (empty when Star)

	Assignments []Assignment // UPDATE ... SET

	Where Expr // optional filter (nil => match all)

	Assert *AssertClause // set only when Kind == StmtAssert
	If     *IfClause     // set only when Kind == StmtIf
}

// IfClause models IF ... THEN ... [ELSIF ... THEN ...] [ELSE ...] END IF.
// Branches holds the IF branch followed by every ELSIF branch, in source order;
// Else is the optional trailing body (nil when there is no ELSE).
type IfClause struct {
	Branches []IfBranch
	Else     []*Statement
}

// IfBranch is one IF/ELSIF arm: a condition built from [NOT] EXISTS terms and
// the statements to run when it holds.
type IfBranch struct {
	Cond Expr
	Body []*Statement
}

// Exists is a subquery existence test, EXISTS ( <select> ). It is only valid in
// an IF condition (negation is expressed by wrapping it in Not), never in WHERE.
type Exists struct{ Select *Statement }

func (*Exists) isExpr() {}

// AssertClause wraps an inner SELECT with an expectation and an optional typed
// failure (code + message) used to build a validation error when the
// expectation is not met.
type AssertClause struct {
	Expect  ExpectKind
	Select  *Statement // the inner SELECT (read-only)
	Code    string     // optional machine-readable code from FAIL '<code>'
	Message string     // optional human message from FAIL '<code>' '<message>'
}

// TableRef references a Kubernetes resource. Resource holds the raw resource
// argument exactly as written (e.g. "pods", "deployments.apps",
// "moduleconfigs.v1alpha1.deckhouse.io"); resolution to a GVR happens later.
type TableRef struct {
	Resource string
	Alias    string
}

// JoinClause models a single equi-join: FROM <left> JOIN <right> ON l == r.
type JoinClause struct {
	Right TableRef
	On    *JoinCond
}

// JoinCond is the ON predicate of a join, restricted to an equality of two
// field references (a hash join can be used).
type JoinCond struct {
	Left  *Field
	Right *Field
}

// ColumnRef is a single projected column.
type ColumnRef struct {
	Field *Field
	Alias string
}

// Assignment is a single SET target = value pair.
type Assignment struct {
	Field *Field
	Value *Lit
}

// Expr is any boolean/comparison expression node.
type Expr interface{ isExpr() }

// Field is a dotted field path. When a join is present, Table holds the resolved
// table alias (the first path segment), and Path holds the remaining segments.
type Field struct {
	Table string
	Path  []string
}

func (*Field) isExpr() {}

// String returns the canonical dotted representation used for output headers.
// It is computed lazily (only when a header is needed) to keep parsing
// allocation-light; WHERE-clause fields never pay for it.
func (f *Field) String() string {
	if f.Table != "" {
		return f.Table + "." + joinPath(f.Path)
	}
	return joinPath(f.Path)
}

// LitKind classifies a literal value.
type LitKind uint8

const (
	LitString LitKind = iota
	LitInt
	LitFloat
	LitBool
	LitNull
)

// Lit is a literal scalar value, pre-parsed to its native type to avoid
// re-parsing during evaluation.
type Lit struct {
	Kind  LitKind
	Str   string
	Int   int64
	Float float64
	Bool  bool
}

func (*Lit) isExpr() {}

// Compare is a binary comparison: Left <op> Right.
// Op is one of EQ, NEQ, LT, LTE, GT, GTE, LIKE, ILIKE.
type Compare struct {
	Op    Kind
	Left  Expr
	Right Expr
}

func (*Compare) isExpr() {}

// Logic is a short-circuit boolean combination (AND/OR).
type Logic struct {
	Op    Kind // AND or OR
	Left  Expr
	Right Expr
}

func (*Logic) isExpr() {}

// Not negates its operand.
type Not struct{ X Expr }

func (*Not) isExpr() {}

// In models <field> IN (v1, v2, ...). Negate covers NOT IN.
type In struct {
	Field  Expr
	Values []*Lit
	Negate bool
}

func (*In) isExpr() {}

// IsNull models <field> IS [NOT] NULL.
type IsNull struct {
	Field  Expr
	Negate bool
}

func (*IsNull) isExpr() {}

// joinPath concatenates path segments with dots. Used only for display.
func joinPath(p []string) string {
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
