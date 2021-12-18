package sortutil

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func popRune(str *string) rune {
	r, sz := utf8.DecodeRuneInString(*str)
	if sz == 0 {
		return utf8.RuneError
	}

	*str = (*str)[sz:]
	return r
}

// LessFold returns true if i < j case-insensitive. See StrcmpFold.
func LessFold(i, j string) bool {
	return CmpFold(i, j) == -1
}

// CmpFold compares 2 strings in a case-insensitive manner. If the string is
// prefixed with !, then it's put to last.
func CmpFold(i, j string) int {
	for {
		ir := popRune(&i)
		jr := popRune(&j)

		if ir == utf8.RuneError || jr == utf8.RuneError {
			if i == "" && j != "" {
				// len(i) < len(j)
				return -1
			}
			if i != "" && j == "" {
				// len(i) > len(j)
				return 1
			}
			return 0
		}

		if ir == '!' {
			return 1 // put last
		}

		if jr == '!' {
			return -1 // put last
		}

		if eq := compareRuneFold(ir, jr); eq != 0 {
			return eq
		}
	}
}

func compareRuneFold(i, j rune) int {
	if i == j {
		return 0
	}

	li := unicode.ToLower(i)
	lj := unicode.ToLower(j)

	if li != lj {
		if li < lj {
			return -1
		}
		return 1
	}

	if i < j {
		return -1
	}
	return 1
}

// ContainsFold is a case-insensitive version of strings.Contains.
func ContainsFold(s, substr string) bool {
	// TODO: faster impl.
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
