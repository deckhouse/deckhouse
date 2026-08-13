package sql

// Lexer is a pull-based scanner over a SQL source string. Unlike the
// channel/goroutine design in text/template/parse, tokens are produced on demand
// via Next, which keeps it allocation-free and CPU-cache friendly.
type Lexer struct {
	src string
	pos int // current scan offset
}

// NewLexer returns a lexer over src. The string is not copied.
func NewLexer(src string) *Lexer {
	return &Lexer{src: src}
}

const eofByte = 0

// peek returns the current byte without consuming it, or eofByte at end.
func (l *Lexer) peek() byte {
	if l.pos >= len(l.src) {
		return eofByte
	}
	return l.src[l.pos]
}

func (l *Lexer) peek2() byte {
	if l.pos+1 >= len(l.src) {
		return eofByte
	}
	return l.src[l.pos+1]
}

// Next scans and returns the next token.
func (l *Lexer) Next() Token {
	l.skipSpaceAndComments()
	start := l.pos
	c := l.peek()
	switch {
	case c == eofByte:
		return Token{Kind: EOF, Pos: start}
	case isIdentStart(c):
		return l.lexIdent()
	case isDigit(c):
		return l.lexNumber()
	case c == '-' && isDigit(l.peek2()):
		return l.lexNumber()
	case c == '\'':
		return l.lexString()
	}

	// punctuation / operators
	l.pos++
	switch c {
	case '*':
		return Token{Kind: STAR, Val: l.src[start:l.pos], Pos: start}
	case ',':
		return Token{Kind: COMMA, Val: l.src[start:l.pos], Pos: start}
	case '.':
		// a dot directly followed by a digit is a fractional number (.5)
		if isDigit(l.peek()) {
			l.pos = start
			return l.lexNumber()
		}
		return Token{Kind: DOT, Val: l.src[start:l.pos], Pos: start}
	case '(':
		return Token{Kind: LPAREN, Val: l.src[start:l.pos], Pos: start}
	case ')':
		return Token{Kind: RPAREN, Val: l.src[start:l.pos], Pos: start}
	case ';':
		return Token{Kind: SEMICOLON, Val: l.src[start:l.pos], Pos: start}
	case '=':
		if l.peek() == '=' {
			l.pos++
		}
		return Token{Kind: EQ, Val: l.src[start:l.pos], Pos: start}
	case '!':
		if l.peek() == '=' {
			l.pos++
			return Token{Kind: NEQ, Val: l.src[start:l.pos], Pos: start}
		}
		return Token{Kind: ILLEGAL, Val: l.src[start:l.pos], Pos: start}
	case '<':
		switch l.peek() {
		case '=':
			l.pos++
			return Token{Kind: LTE, Val: l.src[start:l.pos], Pos: start}
		case '>':
			l.pos++
			return Token{Kind: NEQ, Val: l.src[start:l.pos], Pos: start}
		}
		return Token{Kind: LT, Val: l.src[start:l.pos], Pos: start}
	case '>':
		if l.peek() == '=' {
			l.pos++
			return Token{Kind: GTE, Val: l.src[start:l.pos], Pos: start}
		}
		return Token{Kind: GT, Val: l.src[start:l.pos], Pos: start}
	}
	return Token{Kind: ILLEGAL, Val: l.src[start:l.pos], Pos: start}
}

func (l *Lexer) skipSpaceAndComments() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			l.pos++
		case c == '-' && l.peek2() == '-':
			// line comment until end of line
			l.pos += 2
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '/' && l.peek2() == '*':
			// block comment
			l.pos += 2
			for l.pos < len(l.src) {
				if l.src[l.pos] == '*' && l.peek2() == '/' {
					l.pos += 2
					break
				}
				l.pos++
			}
		default:
			return
		}
	}
}

func (l *Lexer) lexIdent() Token {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	word := l.src[start:l.pos]
	if k, ok := lookupKeyword(word); ok {
		return Token{Kind: k, Val: word, Pos: start}
	}
	return Token{Kind: IDENT, Val: word, Pos: start}
}

func (l *Lexer) lexNumber() Token {
	start := l.pos
	if l.pos < len(l.src) && l.src[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	return Token{Kind: NUMBER, Val: l.src[start:l.pos], Pos: start}
}

// lexString scans a single-quoted string. The returned Val excludes the quotes.
// SQL escaping of a quote is '' (doubled). When no escapes are present the value
// aliases the source directly (zero-copy); otherwise it is unquoted lazily by
// the parser via Unquote.
func (l *Lexer) lexString() Token {
	start := l.pos
	l.pos++ // opening quote
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\'' {
			if l.peek2() == '\'' { // escaped quote
				l.pos += 2
				continue
			}
			inner := l.src[start+1 : l.pos]
			l.pos++ // closing quote
			return Token{Kind: STRING, Val: inner, Pos: start}
		}
		l.pos++
	}
	return Token{Kind: ILLEGAL, Val: l.src[start:l.pos], Pos: start}
}

// lookupKeyword performs an allocation-free, case-insensitive keyword lookup by
// upper-casing into a stack buffer. The map lookup over string(buf[:n]) is
// optimized by the compiler to avoid a heap allocation.
func lookupKeyword(word string) (Kind, bool) {
	const maxKeyword = 8 // longest keyword length (e.g. "DELETE", "SELECT")
	if len(word) > maxKeyword {
		return 0, false
	}
	var buf [maxKeyword]byte
	for i := 0; i < len(word); i++ {
		c := word[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		buf[i] = c
	}
	k, ok := keywordLookup[string(buf[:len(word)])]
	return k, ok
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '-' || c == '/'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
