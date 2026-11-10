package gitignore

import "bytes"

// wildmatch flags, as in git's wildmatch.h.
const (
	wmCaseFold = 1 << iota
	wmPathname
)

// wildmatch results, as in git's wildmatch.c. The two abort results let a
// failed "*" stop retrying positions that cannot match either, which keeps
// matching linear for patterns with many stars.
const (
	wmMatch = iota
	wmNoMatch
	wmAbortAll
	wmAbortToStarStar
)

// wildmatch reports whether text matches the glob pattern with git's
// wildmatch semantics, which ignore files and includeIf gitdir use. It is a
// port of dowild in git's wildmatch.c rather than path.Match or doublestar,
// because those differ from git in ways that change which files are ignored:
// "**" that does not backtrack, "[!a]" read as a class of "!" and "a", and
// braces read as alternation.
func wildmatch(pattern, text string, flags int) bool {
	return dowild([]byte(pattern), []byte(text), flags) == wmMatch
}

func dowild(p, text []byte, flags int) int {
	at := func(b []byte, i int) byte {
		if i < len(b) {
			return b[i]
		}
		return 0
	}
	fold := func(c byte) byte {
		if flags&wmCaseFold != 0 && 'A' <= c && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}
	pi, ti := 0, 0
	for ; pi < len(p); pi, ti = pi+1, ti+1 {
		pc := fold(p[pi])
		tc := fold(at(text, ti))
		if tc == 0 && pc != '*' {
			return wmAbortAll
		}
		switch pc {
		case '\\':
			// Literal match with the following character. A trailing
			// backslash compares with NUL and so fails below.
			pi++
			pc = fold(at(p, pi))
			if tc != pc {
				return wmNoMatch
			}
		case '?':
			if flags&wmPathname != 0 && tc == '/' {
				return wmNoMatch
			}
		case '*':
			var matchSlash bool
			pi++
			if at(p, pi) == '*' {
				prev := pi - 2
				for pi++; at(p, pi) == '*'; pi++ {
				}
				if (prev < 0 || p[prev] == '/') &&
					(pi >= len(p) || p[pi] == '/' || (p[pi] == '\\' && at(p, pi+1) == '/')) {
					// "**/" may match no directory at all, so that
					// foo/**/bar matches foo/bar as well as foo/a/bar.
					if at(p, pi) == '/' && dowild(p[pi+1:], text[ti:], flags) == wmMatch {
						return wmMatch
					}
					matchSlash = true
				}
			} else {
				// Without wmPathname a single "*" also crosses slashes.
				matchSlash = flags&wmPathname == 0
			}
			if pi >= len(p) {
				// A trailing "**" matches everything, a trailing "*" only
				// what has no more slashes.
				if !matchSlash && bytes.IndexByte(text[ti:], '/') >= 0 {
					return wmNoMatch
				}
				return wmMatch
			}
			if !matchSlash && p[pi] == '/' {
				// A single "*" followed by a slash matches the rest of
				// the current directory name.
				slash := bytes.IndexByte(text[ti:], '/')
				if slash < 0 {
					return wmNoMatch
				}
				ti += slash
				// The outer loop consumes the slash in both.
				continue
			}
			for tc != 0 {
				// Skip ahead to the next occurrence of a literal that
				// follows the star, since everything before it must belong
				// to the star. Without matchSlash it cannot pass a slash.
				if pl := at(p, pi); !isGlobSpecial(pl) {
					pl = fold(pl)
					for tc = fold(at(text, ti)); tc != 0 && (matchSlash || tc != '/'); tc = fold(at(text, ti)) {
						if tc == pl {
							break
						}
						ti++
					}
					if tc != pl {
						if matchSlash {
							return wmAbortAll
						}
						return wmAbortToStarStar
					}
				}
				if m := dowild(p[pi:], text[ti:], flags); m != wmNoMatch {
					if !matchSlash || m != wmAbortToStarStar {
						return m
					}
				} else if !matchSlash && tc == '/' {
					return wmAbortToStarStar
				}
				ti++
				tc = fold(at(text, ti))
			}
			return wmAbortAll
		case '[':
			pi++
			pc = at(p, pi)
			if pc == '^' {
				pc = '!'
			}
			negated := pc == '!'
			if negated {
				pi++
				pc = at(p, pi)
			}
			var prev byte
			matched := false
			for {
				if pc == 0 {
					return wmAbortAll
				}
				switch {
				case pc == '\\':
					pi++
					pc = at(p, pi)
					if pc == 0 {
						return wmAbortAll
					}
					if tc == fold(pc) {
						matched = true
					}
				case pc == '-' && prev != 0 && at(p, pi+1) != 0 && at(p, pi+1) != ']':
					pi++
					pc = at(p, pi)
					if pc == '\\' {
						pi++
						pc = at(p, pi)
						if pc == 0 {
							return wmAbortAll
						}
					}
					if inRange(tc, prev, pc) || (flags&wmCaseFold != 0 && inRange(upper(tc), prev, pc)) {
						matched = true
					}
					pc = 0 // so that prev is reset below
				case pc == '[' && at(p, pi+1) == ':':
					start := pi + 2
					end := start
					for end < len(p) && p[end] != ']' {
						end++
					}
					if end >= len(p) {
						return wmAbortAll
					}
					if end-start < 1 || p[end-1] != ':' {
						// No ":]", so the "[" is an ordinary member.
						if tc == '[' {
							matched = true
						}
						break
					}
					class, ok := charClass(string(p[start:end-1]), tc, flags)
					if !ok {
						return wmAbortAll
					}
					if class {
						matched = true
					}
					pi = end
					pc = 0
				default:
					if tc == fold(pc) {
						matched = true
					}
				}
				prev = pc
				pi++
				pc = at(p, pi)
				if pc == ']' {
					break
				}
			}
			if matched == negated || (flags&wmPathname != 0 && tc == '/') {
				return wmNoMatch
			}
		default:
			if tc != pc {
				return wmNoMatch
			}
		}
	}
	if ti < len(text) {
		return wmNoMatch
	}
	return wmMatch
}

func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

func inRange(c, lo, hi byte) bool {
	return lo <= c && c <= hi
}

func upper(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

// charClass evaluates a POSIX character class such as [:alpha:]. ok is false
// for an unknown class name, which git treats as a malformed pattern.
func charClass(name string, c byte, flags int) (in, ok bool) {
	isUpper := 'A' <= c && c <= 'Z'
	isLower := 'a' <= c && c <= 'z'
	isDigit := '0' <= c && c <= '9'
	isAlpha := isUpper || isLower
	switch name {
	case "alnum":
		return isAlpha || isDigit, true
	case "alpha":
		return isAlpha, true
	case "blank":
		return c == ' ' || c == '\t', true
	case "cntrl":
		return c < 0x20 || c == 0x7f, true
	case "digit":
		return isDigit, true
	case "graph":
		return 0x21 <= c && c <= 0x7e, true
	case "lower":
		return isLower || (flags&wmCaseFold != 0 && isUpper), true
	case "print":
		return 0x20 <= c && c <= 0x7e, true
	case "punct":
		return 0x21 <= c && c <= 0x7e && !isAlpha && !isDigit, true
	case "space":
		return c == ' ' || ('\t' <= c && c <= '\r'), true
	case "upper":
		return isUpper || (flags&wmCaseFold != 0 && isLower), true
	case "xdigit":
		return isDigit || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F'), true
	}
	return false, false
}
