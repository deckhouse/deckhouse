package d8sql

// compileLike turns a SQL LIKE/ILIKE pattern into a matcher. The pattern is
// pre-processed once (lower-cased for ILIKE); the returned closure performs an
// allocation-free, backtracking wildcard match per call.
//
// Supported wildcards follow PostgreSQL semantics:
//
//	%  matches any sequence of characters (including empty)
//	_  matches exactly one character
func compileLike(pattern string, ci bool) func(string) bool {
	pat := pattern
	if ci {
		pat = asciiLower(pattern)
	}

	// Fast paths for the overwhelmingly common shapes avoid the generic
	// backtracking loop entirely.
	if !hasWildcard(pat) {
		if ci {
			return func(s string) bool { return equalFoldASCII(s, pat) }
		}
		return func(s string) bool { return s == pat }
	}
	if len(pat) >= 2 && pat[0] == '%' && pat[len(pat)-1] == '%' {
		mid := pat[1 : len(pat)-1]
		if !hasWildcard(mid) {
			if ci {
				return func(s string) bool { return containsFold(s, mid) }
			}
			return func(s string) bool { return indexOf(s, mid) >= 0 }
		}
	}

	return func(s string) bool { return likeMatch(pat, s, ci) }
}

func hasWildcard(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] == '%' || p[i] == '_' {
			return true
		}
	}
	return false
}

// likeMatch is the classic linear-time wildcard matcher with O(1) extra space.
// When ci is true, pat is already lower-cased and only s is folded on the fly.
func likeMatch(pat, s string, ci bool) bool {
	var sx, px int
	starIdx, matchIdx := -1, 0
	for sx < len(s) {
		if px < len(pat) && (pat[px] == '_' || eqByte(pat[px], s[sx], ci)) {
			sx++
			px++
		} else if px < len(pat) && pat[px] == '%' {
			starIdx = px
			matchIdx = sx
			px++
		} else if starIdx != -1 {
			px = starIdx + 1
			matchIdx++
			sx = matchIdx
		} else {
			return false
		}
	}
	for px < len(pat) && pat[px] == '%' {
		px++
	}
	return px == len(pat)
}

func eqByte(p, c byte, ci bool) bool {
	if p == c {
		return true
	}
	if ci {
		return lowerByte(c) == p
	}
	return false
}

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func asciiLower(s string) string {
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		b[i] = lowerByte(s[i])
	}
	return string(b)
}

func equalFoldASCII(s, lowered string) bool {
	if len(s) != len(lowered) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if lowerByte(s[i]) != lowered[i] {
			return false
		}
	}
	return true
}

// indexOf is a tiny substring search (avoids importing strings for the hot path).
func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}
	last := len(s) - len(sub)
	for i := 0; i <= last; i++ {
		if s[i] == sub[0] && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func containsFold(s, loweredSub string) bool {
	if len(loweredSub) == 0 {
		return true
	}
	if len(loweredSub) > len(s) {
		return false
	}
	last := len(s) - len(loweredSub)
	for i := 0; i <= last; i++ {
		if lowerByte(s[i]) == loweredSub[0] && equalFoldASCII(s[i:i+len(loweredSub)], loweredSub) {
			return true
		}
	}
	return false
}
