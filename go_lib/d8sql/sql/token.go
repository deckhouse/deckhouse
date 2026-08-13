package sql

// Kind is the lexical category of a token.
type Kind uint8

const (
	EOF Kind = iota
	ILLEGAL

	IDENT  // unquoted identifier: pods, metadata, status
	STRING // single-quoted literal: 'Running'
	NUMBER // numeric literal: 1, 0, 3.14

	// keywords
	SELECT
	FROM
	WHERE
	UPDATE
	SET
	DELETE
	AND
	OR
	NOT
	LIKE
	ILIKE
	JOIN
	ON
	AS
	IS
	IN
	NULL
	TRUE
	FALSE
	ASSERT
	EMPTY
	FAIL
	IF
	THEN
	ELSIF
	ELSE
	END
	EXISTS
	INSERT
	INTO

	// punctuation
	STAR      // *
	COMMA     // ,
	DOT       // .
	LPAREN    // (
	RPAREN    // )
	SEMICOLON // ;

	// operators
	EQ  // = or ==
	NEQ // != or <>
	LT  // <
	LTE // <=
	GT  // >
	GTE // >=
)

// Token is a single lexical unit. Val aliases into the source string, so the
// lexer performs no allocations while scanning.
type Token struct {
	Kind Kind
	Val  string
	Pos  int // byte offset of the token start in the source
}

var kindNames = [...]string{
	EOF:       "EOF",
	ILLEGAL:   "ILLEGAL",
	IDENT:     "IDENT",
	STRING:    "STRING",
	NUMBER:    "NUMBER",
	SELECT:    "SELECT",
	FROM:      "FROM",
	WHERE:     "WHERE",
	UPDATE:    "UPDATE",
	SET:       "SET",
	DELETE:    "DELETE",
	AND:       "AND",
	OR:        "OR",
	NOT:       "NOT",
	LIKE:      "LIKE",
	ILIKE:     "ILIKE",
	JOIN:      "JOIN",
	ON:        "ON",
	AS:        "AS",
	IS:        "IS",
	IN:        "IN",
	NULL:      "NULL",
	TRUE:      "TRUE",
	FALSE:     "FALSE",
	ASSERT:    "ASSERT",
	EMPTY:     "EMPTY",
	FAIL:      "FAIL",
	IF:        "IF",
	THEN:      "THEN",
	ELSIF:     "ELSIF",
	ELSE:      "ELSE",
	END:       "END",
	EXISTS:    "EXISTS",
	INSERT:    "INSERT",
	INTO:      "INTO",
	STAR:      "*",
	COMMA:     ",",
	DOT:       ".",
	LPAREN:    "(",
	RPAREN:    ")",
	SEMICOLON: ";",
	EQ:        "=",
	NEQ:       "!=",
	LT:        "<",
	LTE:       "<=",
	GT:        ">",
	GTE:       ">=",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "UNKNOWN"
}

// keywordLookup maps the upper-cased identifier to its keyword Kind. It is built
// once at package init and is read-only afterwards.
var keywordLookup = map[string]Kind{
	"SELECT": SELECT,
	"FROM":   FROM,
	"WHERE":  WHERE,
	"UPDATE": UPDATE,
	"SET":    SET,
	"DELETE": DELETE,
	"AND":    AND,
	"OR":     OR,
	"NOT":    NOT,
	"LIKE":   LIKE,
	"ILIKE":  ILIKE,
	"JOIN":   JOIN,
	"ON":     ON,
	"AS":     AS,
	"IS":     IS,
	"IN":     IN,
	"NULL":   NULL,
	"TRUE":   TRUE,
	"FALSE":  FALSE,
	"ASSERT": ASSERT,
	"EMPTY":  EMPTY,
	"FAIL":   FAIL,
	"IF":     IF,
	"THEN":   THEN,
	"ELSIF":  ELSIF,
	"ELSE":   ELSE,
	"END":    END,
	"EXISTS": EXISTS,
	"INSERT": INSERT,
	"INTO":   INTO,
}

// isKeyword reports whether k is a reserved word. The lexer recognizes keywords
// everywhere, so the parser uses this to accept one where an identifier is
// expected — e.g. a field path segment literally named "end".
func (k Kind) isKeyword() bool { return k >= SELECT && k <= INTO }
