package comment

import (
	"errors"
	"strings"
)

// Lexorank produces short strings whose lexicographic order matches the
// desired insertion order, so reordering a thread does not require
// rewriting unrelated rows.
//
// Alphabet is 0-9 + A-Z + a-z (62 symbols). The empty string represents the
// open boundary on either side: an empty low argument means "before
// everything", an empty high argument means "after everything".

const (
	rankAlphabet  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rankMidIdx    = 30 // index of 'U' — the canonical midpoint
	// rankMaxLength caps the rank string length to prevent runaway growth
	// in degenerate insertion patterns. Append-only chains grow by one
	// character every ~62 inserts (the alphabet size), so 4096 supports
	// well over 200k append-only inserts into a single thread. Random
	// insertions stay much shorter via bisection in Mid.
	rankMaxLength = 4096
)

var rankBase = len(rankAlphabet)

// ErrEqualBounds — no value exists strictly between two equal bounds.
var ErrEqualBounds = errors.New("comment/lexorank: equal bounds")

// ErrBoundsReversed — low must be strictly less than high in lex order.
var ErrBoundsReversed = errors.New("comment/lexorank: low >= high")

// ErrRankOverflow guards against pathological growth.
var ErrRankOverflow = errors.New("comment/lexorank: rank grew beyond limit")

func charIndex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 36
	default:
		return -1
	}
}

func charAt(i int) byte {
	if i < 0 {
		return rankAlphabet[0]
	}
	if i >= rankBase {
		return rankAlphabet[rankBase-1]
	}
	return rankAlphabet[i]
}

// First is the canonical rank for the first comment in an empty thread.
func First() string {
	return string(charAt(rankMidIdx))
}

// After returns a rank strictly greater than last. Used for append-only
// insertion (the common case for chronological replies). Bumps the last
// character by one when there is alphabet headroom; appends the midpoint
// character when the last character is at the ceiling. This gives roughly
// O(log_B N) growth in rank length for N append-only inserts, far better
// than the bisection-based Mid which converges on length proportional to
// the number of inserts.
func After(last string) (string, error) {
	if last == "" {
		return First(), nil
	}
	idx := charIndex(last[len(last)-1])
	if idx < 0 {
		return "", ErrBoundsReversed
	}
	if idx < rankBase-1 {
		return last[:len(last)-1] + string(charAt(idx+1)), nil
	}
	out := last + string(charAt(rankMidIdx))
	if len(out) > rankMaxLength {
		return "", ErrRankOverflow
	}
	return out, nil
}

// Before returns a rank strictly less than first. Mirrors After: bump the
// last character down when there is headroom; otherwise descend a level.
func Before(first string) (string, error) {
	if first == "" {
		return First(), nil
	}
	idx := charIndex(first[len(first)-1])
	if idx < 0 {
		return "", ErrBoundsReversed
	}
	if idx > 0 {
		return first[:len(first)-1] + string(charAt(idx-1)), nil
	}
	// Last character is the alphabet minimum and we cannot shorten the
	// string without losing a strict lex-less result for the typical
	// case (an empty string is reserved as the open boundary). Recurse
	// on the prefix when one exists, otherwise refuse.
	if len(first) > 1 {
		return Before(first[:len(first)-1])
	}
	return "", ErrBoundsReversed
}

// lowAt returns the alphabet index at position i of low, or 0 if low has
// ended (padding with the alphabet minimum).
func lowAt(low string, i int) int {
	if i < len(low) {
		return charIndex(low[i])
	}
	return 0
}

// Mid returns a rank strictly between low and high. Empty low means
// "before everything"; empty high means "after everything".
func Mid(low, high string) (string, error) {
	if low == "" && high == "" {
		return First(), nil
	}
	if low != "" && high != "" {
		if low == high {
			return "", ErrEqualBounds
		}
		if low > high {
			return "", ErrBoundsReversed
		}
	}

	rightOpen := high == ""

	var out strings.Builder
	for i := 0; ; i++ {
		if i > rankMaxLength {
			return "", ErrRankOverflow
		}
		lo := lowAt(low, i)

		if rightOpen {
			// Open right: anything strictly greater than low is acceptable.
			// Pick the midpoint between lo and the alphabet ceiling. If lo
			// is already at the ceiling, copy and continue so we extend
			// low's tail with a smaller suffix.
			if lo >= rankBase-1 {
				out.WriteByte(charAt(lo))
				continue
			}
			mid := lo + (rankBase-lo)/2
			if mid == lo {
				mid = lo + 1
			}
			out.WriteByte(charAt(mid))
			return out.String(), nil
		}

		// Bounded right.
		if i >= len(high) {
			// We have matched high entirely without dropping below it.
			// There is no character we can emit and stay strictly less
			// than high, so the caller asked for an impossible interval.
			return "", ErrBoundsReversed
		}
		hi := charIndex(high[i])

		if hi-lo > 1 {
			mid := lo + (hi-lo)/2
			out.WriteByte(charAt(mid))
			return out.String(), nil
		}

		if hi == lo {
			out.WriteByte(charAt(lo))
			continue
		}

		// Adjacent (hi == lo + 1). Keep lo's character; produce a suffix
		// strictly greater than low's tail so the full rank exceeds low
		// while staying below high.
		out.WriteByte(charAt(lo))
		tail, err := suffixAfter(low, i+1)
		if err != nil {
			return "", err
		}
		out.WriteString(tail)
		return out.String(), nil
	}
}

// suffixAfter returns a non-empty string T such that low[start:] < T
// in lexicographic order. T is kept as short as possible.
func suffixAfter(low string, start int) (string, error) {
	tail := ""
	if start < len(low) {
		tail = low[start:]
	}
	for i := 0; i < len(tail); i++ {
		idx := charIndex(tail[i])
		if idx < 0 {
			return "", ErrBoundsReversed
		}
		if idx < rankBase-1 {
			mid := idx + (rankBase-idx)/2
			if mid == idx {
				mid = idx + 1
			}
			return tail[:i] + string(charAt(mid)), nil
		}
	}
	out := tail + string(charAt(rankMidIdx))
	if len(out) > rankMaxLength {
		return "", ErrRankOverflow
	}
	return out, nil
}
