package sql

import (
	"fmt"
	"strconv"
)

// Parser is a recursive-descent parser with a single token of lookahead.
type Parser struct {
	lex *Lexer
	cur Token
}

// NewParser returns a parser over src.
func NewParser(src string) *Parser {
	p := &Parser{lex: NewLexer(src)}
	p.cur = p.lex.Next()
	return p
}

// Parse parses exactly one statement and requires EOF (a trailing semicolon is
// allowed) afterwards.
func Parse(src string) (*Statement, error) {
	p := NewParser(src)
	st, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	if p.cur.Kind == SEMICOLON {
		p.advance()
	}
	if p.cur.Kind != EOF {
		return nil, p.errf("unexpected %s after statement", p.cur.Kind)
	}
	return st, nil
}

// ParseMany parses one or more semicolon-separated statements.
func ParseMany(src string) ([]*Statement, error) {
	p := NewParser(src)
	var stmts []*Statement
	for p.cur.Kind != EOF {
		if p.cur.Kind == SEMICOLON {
			p.advance()
			continue
		}
		st, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, st)
		if p.cur.Kind == SEMICOLON {
			p.advance()
		} else if p.cur.Kind != EOF {
			return nil, p.errf("expected ';' or end of input, got %s", p.cur.Kind)
		}
	}
	if len(stmts) == 0 {
		return nil, fmt.Errorf("d8sql: empty query")
	}
	return stmts, nil
}

func (p *Parser) advance() {
	p.cur = p.lex.Next()
}

func (p *Parser) expect(k Kind) (Token, error) {
	if p.cur.Kind != k {
		return Token{}, p.errf("expected %s, got %s", k, p.cur.Kind)
	}
	t := p.cur
	p.advance()
	return t, nil
}

func (p *Parser) errf(format string, args ...any) error {
	return fmt.Errorf("d8sql: parse error at offset %d: %s", p.cur.Pos, fmt.Sprintf(format, args...))
}

func (p *Parser) parseStatement() (*Statement, error) {
	switch p.cur.Kind {
	case SELECT:
		return p.parseSelect()
	case UPDATE:
		return p.parseUpdate()
	case DELETE:
		return p.parseDelete()
	case ASSERT:
		return p.parseAssert()
	case IF:
		return p.parseIf()
	default:
		return nil, p.errf("expected SELECT, UPDATE, DELETE, ASSERT or IF, got %s", p.cur.Kind)
	}
}

// parseIf parses:
//
//	IF <cond> THEN <stmts> [ELSIF <cond> THEN <stmts>]... [ELSE <stmts>] END IF
//
// where <cond> is a boolean combination of [NOT] EXISTS ( <select> ) terms and
// the bodies are semicolon-separated statements (nested IFs included).
func (p *Parser) parseIf() (*Statement, error) {
	ic := &IfClause{}
	for {
		p.advance() // IF or ELSIF
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(THEN); err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		ic.Branches = append(ic.Branches, IfBranch{Cond: cond, Body: body})
		if p.cur.Kind != ELSIF {
			break
		}
	}
	if p.cur.Kind == ELSE {
		p.advance()
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		ic.Else = body
	}
	if _, err := p.expect(END); err != nil {
		return nil, err
	}
	if _, err := p.expect(IF); err != nil {
		return nil, err
	}
	return &Statement{Kind: StmtIf, If: ic}, nil
}

// parseBlock parses the semicolon-separated statements of an IF body, stopping
// (without consuming it) at ELSIF, ELSE or END.
func (p *Parser) parseBlock() ([]*Statement, error) {
	var body []*Statement
	for {
		switch p.cur.Kind {
		case ELSIF, ELSE, END:
			return body, nil
		case EOF:
			return nil, p.errf("unexpected end of input, expected END IF")
		case SEMICOLON:
			p.advance()
			continue
		}
		st, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, st)
		switch p.cur.Kind {
		case SEMICOLON:
			p.advance()
		case ELSIF, ELSE, END:
		default:
			return nil, p.errf("expected ';' after statement in IF body, got %s", p.cur.Kind)
		}
	}
}

// parseExists parses EXISTS ( <select> ).
func (p *Parser) parseExists() (Expr, error) {
	p.advance() // EXISTS
	if _, err := p.expect(LPAREN); err != nil {
		return nil, err
	}
	if p.cur.Kind != SELECT {
		return nil, p.errf("EXISTS requires a SELECT subquery, got %s", p.cur.Kind)
	}
	inner, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(RPAREN); err != nil {
		return nil, err
	}
	return &Exists{Select: inner}, nil
}

// parseAssert parses:
//
//	ASSERT EMPTY     ( <select> ) [ FAIL '<code>' ['<message>'] ]
//	ASSERT NOT EMPTY ( <select> ) [ FAIL '<code>' ['<message>'] ]
func (p *Parser) parseAssert() (*Statement, error) {
	p.advance() // ASSERT

	var expect ExpectKind
	switch p.cur.Kind {
	case EMPTY:
		expect = ExpectEmpty
		p.advance()
	case NOT:
		p.advance()
		if _, err := p.expect(EMPTY); err != nil {
			return nil, err
		}
		expect = ExpectNotEmpty
	default:
		return nil, p.errf("expected EMPTY or NOT EMPTY after ASSERT, got %s", p.cur.Kind)
	}

	if _, err := p.expect(LPAREN); err != nil {
		return nil, err
	}
	if p.cur.Kind != SELECT {
		return nil, p.errf("ASSERT requires a SELECT subquery, got %s", p.cur.Kind)
	}
	inner, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(RPAREN); err != nil {
		return nil, err
	}

	ac := &AssertClause{Expect: expect, Select: inner}
	if p.cur.Kind == FAIL {
		p.advance()
		code, err := p.expect(STRING)
		if err != nil {
			return nil, err
		}
		ac.Code = unquote(code.Val)
		if p.cur.Kind == STRING {
			ac.Message = unquote(p.cur.Val)
			p.advance()
		}
	}
	return &Statement{Kind: StmtAssert, Assert: ac}, nil
}

func (p *Parser) parseSelect() (*Statement, error) {
	st := &Statement{Kind: StmtSelect}
	p.advance() // SELECT

	if p.cur.Kind == STAR {
		st.Star = true
		p.advance()
	} else {
		for {
			col, err := p.parseColumn()
			if err != nil {
				return nil, err
			}
			st.Columns = append(st.Columns, col)
			if p.cur.Kind != COMMA {
				break
			}
			p.advance()
		}
	}

	if _, err := p.expect(FROM); err != nil {
		return nil, err
	}
	tr, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	st.Table = tr

	if p.cur.Kind == JOIN {
		jc, err := p.parseJoin()
		if err != nil {
			return nil, err
		}
		st.Join = jc
	}

	if p.cur.Kind == WHERE {
		p.advance()
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		st.Where = w
	}

	if st.Join != nil {
		resolveJoinFields(st)
	}
	return st, nil
}

func (p *Parser) parseUpdate() (*Statement, error) {
	st := &Statement{Kind: StmtUpdate}
	p.advance() // UPDATE
	tr, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	st.Table = tr
	if _, err := p.expect(SET); err != nil {
		return nil, err
	}
	for {
		f, err := p.parseField()
		if err != nil {
			return nil, err
		}
		if p.cur.Kind != EQ {
			return nil, p.errf("expected '=' in assignment, got %s", p.cur.Kind)
		}
		p.advance()
		lit, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		st.Assignments = append(st.Assignments, Assignment{Field: f, Value: lit})
		if p.cur.Kind != COMMA {
			break
		}
		p.advance()
	}
	if p.cur.Kind == WHERE {
		p.advance()
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		st.Where = w
	}
	return st, nil
}

func (p *Parser) parseDelete() (*Statement, error) {
	st := &Statement{Kind: StmtDelete}
	p.advance() // DELETE
	if _, err := p.expect(FROM); err != nil {
		return nil, err
	}
	tr, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	st.Table = tr
	if p.cur.Kind == WHERE {
		p.advance()
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		st.Where = w
	}
	return st, nil
}

func (p *Parser) parseColumn() (ColumnRef, error) {
	f, err := p.parseField()
	if err != nil {
		return ColumnRef{}, err
	}
	col := ColumnRef{Field: f}
	if p.cur.Kind == AS {
		p.advance()
		alias, err := p.expect(IDENT)
		if err != nil {
			return ColumnRef{}, err
		}
		col.Alias = alias.Val
	}
	return col, nil
}

// parseTableRef parses a resource reference. The resource argument is either a
// single-quoted string or a dotted identifier sequence.
func (p *Parser) parseTableRef() (TableRef, error) {
	var tr TableRef
	switch p.cur.Kind {
	case STRING:
		tr.Resource = p.cur.Val
		p.advance()
	case IDENT:
		res := p.cur.Val
		p.advance()
		for p.cur.Kind == DOT {
			p.advance()
			seg, err := p.expectIdentLike()
			if err != nil {
				return tr, err
			}
			res = res + "." + seg
		}
		tr.Resource = res
	default:
		return tr, p.errf("expected resource name, got %s", p.cur.Kind)
	}
	// optional alias: AS ident, or bare ident that is not a clause keyword
	switch p.cur.Kind {
	case AS:
		p.advance()
		alias, err := p.expect(IDENT)
		if err != nil {
			return tr, err
		}
		tr.Alias = alias.Val
	case IDENT:
		tr.Alias = p.cur.Val
		p.advance()
	}
	return tr, nil
}

func (p *Parser) parseJoin() (*JoinClause, error) {
	p.advance() // JOIN
	right, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(ON); err != nil {
		return nil, err
	}
	left, err := p.parseField()
	if err != nil {
		return nil, err
	}
	if p.cur.Kind != EQ {
		return nil, p.errf("only equi-joins are supported; expected '=' or '==', got %s", p.cur.Kind)
	}
	p.advance()
	rf, err := p.parseField()
	if err != nil {
		return nil, err
	}
	return &JoinClause{Right: right, On: &JoinCond{Left: left, Right: rf}}, nil
}

// parseField parses a dotted field path. Segments may be identifiers or
// single-quoted strings (for keys containing dots or slashes).
func (p *Parser) parseField() (*Field, error) {
	if p.cur.Kind != IDENT && p.cur.Kind != STRING {
		return nil, p.errf("expected field name, got %s", p.cur.Kind)
	}
	path := make([]string, 0, 4)
	path = append(path, p.cur.Val)
	p.advance()
	for p.cur.Kind == DOT {
		p.advance()
		// A reserved word is accepted here: object fields may legitimately be
		// named "end", "if" or "set".
		if p.cur.Kind != IDENT && p.cur.Kind != STRING && !p.cur.Kind.isKeyword() {
			return nil, p.errf("expected field segment after '.', got %s", p.cur.Kind)
		}
		path = append(path, p.cur.Val)
		p.advance()
	}
	return &Field{Path: path}, nil
}

// expectIdentLike accepts an identifier or a contextual keyword token used as an
// identifier (group/version segments such as "on" are unlikely but keywords like
// "in" could collide; we accept the literal text).
func (p *Parser) expectIdentLike() (string, error) {
	if p.cur.Kind == IDENT || p.cur.Val != "" {
		v := p.cur.Val
		p.advance()
		return v, nil
	}
	return "", p.errf("expected identifier, got %s", p.cur.Kind)
}

// ---- expression parsing (precedence climbing) ----

func (p *Parser) parseExpr() (Expr, error) { return p.parseOr() }

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.cur.Kind == OR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Logic{Op: OR, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.cur.Kind == AND {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &Logic{Op: AND, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (Expr, error) {
	if p.cur.Kind == NOT {
		p.advance()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &Not{X: x}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expr, error) {
	if p.cur.Kind == EXISTS {
		return p.parseExists()
	}
	if p.cur.Kind == LPAREN {
		p.advance()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(RPAREN); err != nil {
			return nil, err
		}
		return e, nil
	}
	return p.parsePredicate()
}

// parsePredicate parses a single comparison/LIKE/IN/IS NULL predicate.
func (p *Parser) parsePredicate() (Expr, error) {
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}

	negate := false
	if p.cur.Kind == NOT {
		// field NOT IN / NOT LIKE / NOT ILIKE
		p.advance()
		negate = true
	}

	switch p.cur.Kind {
	case EQ, NEQ, LT, LTE, GT, GTE:
		if negate {
			return nil, p.errf("unexpected NOT before %s", p.cur.Kind)
		}
		op := p.cur.Kind
		p.advance()
		right, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		return &Compare{Op: op, Left: left, Right: right}, nil
	case LIKE, ILIKE:
		op := p.cur.Kind
		p.advance()
		pat, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		c := &Compare{Op: op, Left: left, Right: pat}
		if negate {
			return &Not{X: c}, nil
		}
		return c, nil
	case IN:
		p.advance()
		if _, err := p.expect(LPAREN); err != nil {
			return nil, err
		}
		var vals []*Lit
		for {
			lit, err := p.parseLiteral()
			if err != nil {
				return nil, err
			}
			vals = append(vals, lit)
			if p.cur.Kind != COMMA {
				break
			}
			p.advance()
		}
		if _, err := p.expect(RPAREN); err != nil {
			return nil, err
		}
		return &In{Field: left, Values: vals, Negate: negate}, nil
	case IS:
		p.advance()
		isNeg := negate
		if p.cur.Kind == NOT {
			p.advance()
			isNeg = !isNeg
		}
		if _, err := p.expect(NULL); err != nil {
			return nil, err
		}
		return &IsNull{Field: left, Negate: isNeg}, nil
	}
	if negate {
		return nil, p.errf("expected IN, LIKE or comparison after NOT, got %s", p.cur.Kind)
	}
	return nil, p.errf("expected comparison operator, got %s", p.cur.Kind)
}

// parseOperand parses either a field reference or a literal value.
func (p *Parser) parseOperand() (Expr, error) {
	switch p.cur.Kind {
	case IDENT:
		return p.parseField()
	case STRING, NUMBER, TRUE, FALSE, NULL:
		return p.parseLiteral()
	default:
		return nil, p.errf("expected field or literal, got %s", p.cur.Kind)
	}
}

func (p *Parser) parseLiteral() (*Lit, error) {
	t := p.cur
	switch t.Kind {
	case STRING:
		p.advance()
		return &Lit{Kind: LitString, Str: unquote(t.Val)}, nil
	case NUMBER:
		p.advance()
		if i, err := strconv.ParseInt(t.Val, 10, 64); err == nil {
			return &Lit{Kind: LitInt, Int: i, Float: float64(i), Str: t.Val}, nil
		}
		f, err := strconv.ParseFloat(t.Val, 64)
		if err != nil {
			return nil, p.errf("invalid number %q", t.Val)
		}
		return &Lit{Kind: LitFloat, Float: f, Str: t.Val}, nil
	case TRUE:
		p.advance()
		return &Lit{Kind: LitBool, Bool: true}, nil
	case FALSE:
		p.advance()
		return &Lit{Kind: LitBool, Bool: false}, nil
	case NULL:
		p.advance()
		return &Lit{Kind: LitNull}, nil
	default:
		return nil, p.errf("expected literal, got %s", t.Kind)
	}
}

// unquote collapses doubled single quotes into one. When there are no
// escapes (the common case) the input is returned unchanged with no allocation.
func unquote(s string) string {
	idx := -1
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '\'' && s[i+1] == '\'' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s
	}
	b := make([]byte, 0, len(s))
	b = append(b, s[:idx]...)
	for i := idx; i < len(s); i++ {
		b = append(b, s[i])
		if s[i] == '\'' && i+1 < len(s) && s[i+1] == '\'' {
			i++
		}
	}
	return string(b)
}

// resolveJoinFields rewrites field references that are prefixed with a table
// alias so that Field.Table holds the alias and Field.Path holds the remainder.
func resolveJoinFields(st *Statement) {
	leftAlias := st.Table.Alias
	if leftAlias == "" {
		leftAlias = st.Table.Resource
	}
	rightAlias := st.Join.Right.Alias
	if rightAlias == "" {
		rightAlias = st.Join.Right.Resource
	}
	split := func(f *Field) {
		if f == nil || len(f.Path) < 2 {
			return
		}
		switch f.Path[0] {
		case leftAlias:
			f.Table = leftAlias
			f.Path = f.Path[1:]
		case rightAlias:
			f.Table = rightAlias
			f.Path = f.Path[1:]
		}
	}
	for i := range st.Columns {
		split(st.Columns[i].Field)
	}
	split(st.Join.On.Left)
	split(st.Join.On.Right)
	walkFields(st.Where, split)
}

func walkFields(e Expr, fn func(*Field)) {
	switch n := e.(type) {
	case nil:
		return
	case *Field:
		fn(n)
	case *Compare:
		walkFields(n.Left, fn)
		walkFields(n.Right, fn)
	case *Logic:
		walkFields(n.Left, fn)
		walkFields(n.Right, fn)
	case *Not:
		walkFields(n.X, fn)
	case *In:
		walkFields(n.Field, fn)
	case *IsNull:
		walkFields(n.Field, fn)
	}
}
