package d8sql

import (
	"github.com/deckhouse/d8sql/sql"
)

// row carries the object(s) a predicate is evaluated against. For single-table
// queries only obj is set; for joins, left and right hold the two sides.
type row struct {
	obj   map[string]any
	left  map[string]any
	right map[string]any
}

// getter extracts a value from a row. ok is false when the path is absent.
type getter func(r *row) (val any, ok bool)

// predicate is a compiled boolean WHERE clause. A nil predicate matches all.
type predicate func(r *row) bool

// compiler turns an AST into closures once per query so that per-object
// evaluation performs no map lookups for dispatch and no allocations.
type compiler struct {
	leftAlias  string
	rightAlias string
}

// compilePredicate compiles a WHERE expression. A nil expression yields nil
// (interpreted as "match everything") to avoid a per-row call.
func (c *compiler) compilePredicate(e sql.Expr) (predicate, error) {
	if e == nil {
		return nil, nil
	}
	switch n := e.(type) {
	case *sql.Logic:
		l, err := c.compilePredicate(n.Left)
		if err != nil {
			return nil, err
		}
		r, err := c.compilePredicate(n.Right)
		if err != nil {
			return nil, err
		}
		if n.Op == sql.AND {
			return func(rw *row) bool { return l(rw) && r(rw) }, nil
		}
		return func(rw *row) bool { return l(rw) || r(rw) }, nil
	case *sql.Not:
		x, err := c.compilePredicate(n.X)
		if err != nil {
			return nil, err
		}
		return func(rw *row) bool { return !x(rw) }, nil
	case *sql.Compare:
		return c.compileCompare(n)
	case *sql.In:
		return c.compileIn(n)
	case *sql.IsNull:
		g, err := c.compileOperand(n.Field)
		if err != nil {
			return nil, err
		}
		neg := n.Negate
		return func(rw *row) bool {
			v, ok := g(rw)
			isNull := !ok || v == nil
			if neg {
				return !isNull
			}
			return isNull
		}, nil
	default:
		return nil, errf("unsupported expression %T in WHERE", e)
	}
}

func (c *compiler) compileCompare(n *sql.Compare) (predicate, error) {
	left, err := c.compileOperand(n.Left)
	if err != nil {
		return nil, err
	}

	if n.Op == sql.LIKE || n.Op == sql.ILIKE {
		lit, ok := n.Right.(*sql.Lit)
		if !ok || lit.Kind != sql.LitString {
			return nil, errf("LIKE pattern must be a string literal")
		}
		m := compileLike(lit.Str, n.Op == sql.ILIKE)
		return func(rw *row) bool {
			v, ok := left(rw)
			if !ok {
				return false
			}
			s, ok := v.(string)
			if !ok {
				return false
			}
			return m(s)
		}, nil
	}

	right, err := c.compileOperand(n.Right)
	if err != nil {
		return nil, err
	}
	op := n.Op
	return func(rw *row) bool {
		l, lok := left(rw)
		if !lok {
			return false
		}
		r, rok := right(rw)
		if !rok {
			return false
		}
		return compareValues(op, l, r)
	}, nil
}

func (c *compiler) compileIn(n *sql.In) (predicate, error) {
	g, err := c.compileOperand(n.Field)
	if err != nil {
		return nil, err
	}
	vals := make([]any, len(n.Values))
	for i, l := range n.Values {
		vals[i] = litValue(l)
	}
	neg := n.Negate
	return func(rw *row) bool {
		v, ok := g(rw)
		if !ok {
			if neg {
				return true
			}
			return false
		}
		found := false
		for _, cand := range vals {
			if valueEqual(v, cand) {
				found = true
				break
			}
		}
		if neg {
			return !found
		}
		return found
	}, nil
}

// compileOperand compiles a field reference or a literal into a getter.
func (c *compiler) compileOperand(e sql.Expr) (getter, error) {
	switch n := e.(type) {
	case *sql.Field:
		return c.compileField(n), nil
	case *sql.Lit:
		v := litValue(n)
		return func(*row) (any, bool) { return v, true }, nil
	default:
		return nil, errf("unsupported operand %T", e)
	}
}

// compileField resolves which side of a join the field reads from and returns a
// closure that walks the (already split) path with no per-call allocation.
func (c *compiler) compileField(f *sql.Field) getter {
	path := f.Path
	switch {
	case f.Table == "" || (f.Table != c.leftAlias && f.Table != c.rightAlias):
		return func(r *row) (any, bool) {
			if r.obj != nil {
				return getPath(r.obj, path)
			}
			// in a join with an unqualified column, default to the left side
			return getPath(r.left, path)
		}
	case f.Table == c.leftAlias:
		return func(r *row) (any, bool) { return getPath(r.left, path) }
	default:
		return func(r *row) (any, bool) { return getPath(r.right, path) }
	}
}

// getPath walks a nested unstructured map following path. It allocates nothing.
func getPath(m map[string]any, path []string) (any, bool) {
	var cur any = m
	for _, key := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mm[key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// litValue converts a literal into its native Go value used during evaluation.
func litValue(l *sql.Lit) any {
	switch l.Kind {
	case sql.LitString:
		return l.Str
	case sql.LitInt:
		return l.Int
	case sql.LitFloat:
		return l.Float
	case sql.LitBool:
		return l.Bool
	default:
		return nil
	}
}

// toFloat returns the numeric value of v, handling the int64/float64 types that
// unstructured JSON decoding produces, plus the int kinds for literals.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

// valueEqual reports semantic equality, comparing numbers numerically and
// strings/bools by value.
func valueEqual(a, b any) bool {
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			return af == bf
		}
		return false
	}
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	}
	return false
}

// compareValues evaluates op between two values. Equality is type-aware; the
// ordering operators apply to numbers and strings.
func compareValues(op sql.Kind, a, b any) bool {
	switch op {
	case sql.EQ:
		return valueEqual(a, b)
	case sql.NEQ:
		return !valueEqual(a, b)
	}
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			switch op {
			case sql.LT:
				return af < bf
			case sql.LTE:
				return af <= bf
			case sql.GT:
				return af > bf
			case sql.GTE:
				return af >= bf
			}
		}
		return false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		switch op {
		case sql.LT:
			return as < bs
		case sql.LTE:
			return as <= bs
		case sql.GT:
			return as > bs
		case sql.GTE:
			return as >= bs
		}
	}
	return false
}
