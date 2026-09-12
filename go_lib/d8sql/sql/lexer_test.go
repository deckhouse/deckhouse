package sql

import "testing"

func tokens(src string) []Token {
	l := NewLexer(src)
	var out []Token
	for {
		t := l.Next()
		out = append(out, t)
		if t.Kind == EOF {
			return out
		}
	}
}

func TestLexerBasic(t *testing.T) {
	got := tokens("SELECT metadata.name FROM pods WHERE status.phase = 'Running';")
	want := []Kind{SELECT, IDENT, DOT, IDENT, FROM, IDENT, WHERE, IDENT, DOT, IDENT, EQ, STRING, SEMICOLON, EOF}
	if len(got) != len(want) {
		t.Fatalf("token count: got %d want %d (%v)", len(got), len(want), got)
	}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s", i, got[i].Kind, k)
		}
	}
}

func TestLexerOperators(t *testing.T) {
	got := tokens("a == b != c <> d <= e >= f < g > h")
	want := []Kind{IDENT, EQ, IDENT, NEQ, IDENT, NEQ, IDENT, LTE, IDENT, GTE, IDENT, LT, IDENT, GT, IDENT, EOF}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s", i, got[i].Kind, k)
		}
	}
}

func TestLexerStringEscape(t *testing.T) {
	got := tokens("'it''s'")
	if got[0].Kind != STRING {
		t.Fatalf("want STRING got %s", got[0].Kind)
	}
	if unquote(got[0].Val) != "it's" {
		t.Errorf("unquote: got %q want %q", unquote(got[0].Val), "it's")
	}
}

func TestLexerKeywordCaseInsensitive(t *testing.T) {
	got := tokens("select Metadata.Name from Pods")
	want := []Kind{SELECT, IDENT, DOT, IDENT, FROM, IDENT, EOF}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s", i, got[i].Kind, k)
		}
	}
}

func TestLexerNumbers(t *testing.T) {
	got := tokens("1 -2 3.14 0")
	want := []Kind{NUMBER, NUMBER, NUMBER, NUMBER, EOF}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s", i, got[i].Kind, k)
		}
	}
	if got[1].Val != "-2" {
		t.Errorf("negative number: got %q", got[1].Val)
	}
}

func TestLexerComments(t *testing.T) {
	got := tokens("SELECT -- comment\n * FROM pods /* block */ ;")
	want := []Kind{SELECT, STAR, FROM, IDENT, SEMICOLON, EOF}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s", i, got[i].Kind, k)
		}
	}
}

func TestLexerGroupQualified(t *testing.T) {
	// dotted resource args and slash-containing label segments
	got := tokens("rolebindings.rbac.authorization.k8s.io")
	want := []Kind{IDENT, DOT, IDENT, DOT, IDENT, DOT, IDENT, DOT, IDENT, EOF}
	for i, k := range want {
		if got[i].Kind != k {
			t.Errorf("token %d: got %s want %s (%q)", i, got[i].Kind, k, got[i].Val)
		}
	}
}
