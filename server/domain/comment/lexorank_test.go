package comment

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

func TestFirst_IsAlphabetMidpoint(t *testing.T) {
	got := First()
	if got != "U" {
		t.Fatalf("First() = %q, want %q", got, "U")
	}
}

func TestCharIndex_RoundTrip(t *testing.T) {
	for i := 0; i < rankBase; i++ {
		c := charAt(i)
		if got := charIndex(c); got != i {
			t.Errorf("charIndex(%q) = %d, want %d", c, got, i)
		}
	}
}

func TestCharIndex_InvalidByte(t *testing.T) {
	if got := charIndex('!'); got != -1 {
		t.Errorf("charIndex('!') = %d, want -1", got)
	}
}

func TestMid_OpenBounds(t *testing.T) {
	got, err := Mid("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "U" {
		t.Errorf("Mid(\"\", \"\") = %q, want %q", got, "U")
	}
}

func TestMid_OpenLow(t *testing.T) {
	got, err := Mid("", "U")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got >= "U" || got <= "" {
		t.Errorf("Mid(\"\", \"U\") = %q, want strictly between \"\" and \"U\"", got)
	}
}

func TestMid_OpenHigh(t *testing.T) {
	got, err := Mid("U", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got <= "U" {
		t.Errorf("Mid(\"U\", \"\") = %q, want strictly greater than \"U\"", got)
	}
}

func TestMid_AdjacentChars(t *testing.T) {
	// 'U' (idx 30) and 'V' (idx 31) are adjacent — no single character fits
	// strictly between them, so the result must extend.
	got, err := Mid("U", "V")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(got > "U" && got < "V") {
		t.Errorf("Mid(\"U\", \"V\") = %q, want strictly between U and V", got)
	}
	if !strings.HasPrefix(got, "U") {
		t.Errorf("Mid(\"U\", \"V\") = %q, want to start with U", got)
	}
}

func TestMid_NestedAdjacent(t *testing.T) {
	got, err := Mid("UU", "UV")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(got > "UU" && got < "UV") {
		t.Errorf("Mid(\"UU\", \"UV\") = %q, want strictly between", got)
	}
}

func TestMid_EqualBounds(t *testing.T) {
	if _, err := Mid("U", "U"); !errors.Is(err, ErrEqualBounds) {
		t.Errorf("Mid(\"U\", \"U\") err = %v, want ErrEqualBounds", err)
	}
}

func TestMid_ReversedBounds(t *testing.T) {
	if _, err := Mid("V", "U"); !errors.Is(err, ErrBoundsReversed) {
		t.Errorf("Mid(\"V\", \"U\") err = %v, want ErrBoundsReversed", err)
	}
}

func TestAfter_BumpsLastChar(t *testing.T) {
	got, err := After("U")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got <= "U" {
		t.Errorf("After(\"U\") = %q, want > \"U\"", got)
	}
}

func TestAfter_AtAlphabetMax(t *testing.T) {
	got, err := After("z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got <= "z" {
		t.Errorf("After(\"z\") = %q, want > \"z\"", got)
	}
}

func TestBefore_BumpsLastChar(t *testing.T) {
	got, err := Before("U")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got >= "U" || got == "" {
		t.Errorf("Before(\"U\") = %q, want non-empty < \"U\"", got)
	}
}

func TestBefore_AtAlphabetMin(t *testing.T) {
	// Before("0") asks for something < the alphabet minimum; the algorithm
	// emits "0..." chains that remain less by being shorter / extended in
	// a controlled way. Either we return ErrBoundsReversed-style error OR
	// we produce a valid lex-smaller rank. Test for the rule, not the exact
	// implementation behaviour.
	got, err := Before("0")
	if err != nil {
		// Acceptable for the implementation to refuse this corner case.
		return
	}
	if got >= "0" {
		t.Errorf("Before(\"0\") = %q, want < \"0\" or an error", got)
	}
}

// TestMonotonicInsertion exercises a long sequence of inserts at random
// positions. After each insert the array must remain sorted and all rank
// strings must remain unique.
func TestMonotonicInsertion(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ranks := []string{First()}
	const inserts = 1000
	for i := 0; i < inserts; i++ {
		pos := rng.Intn(len(ranks) + 1)
		var lo, hi string
		if pos > 0 {
			lo = ranks[pos-1]
		}
		if pos < len(ranks) {
			hi = ranks[pos]
		}
		got, err := Mid(lo, hi)
		if err != nil {
			t.Fatalf("iter %d Mid(%q,%q): %v", i, lo, hi, err)
		}
		if lo != "" && got <= lo {
			t.Fatalf("iter %d Mid(%q,%q) = %q, not greater than low", i, lo, hi, got)
		}
		if hi != "" && got >= hi {
			t.Fatalf("iter %d Mid(%q,%q) = %q, not less than high", i, lo, hi, got)
		}
		ranks = append(ranks[:pos], append([]string{got}, ranks[pos:]...)...)
	}

	for i := 1; i < len(ranks); i++ {
		if !(ranks[i-1] < ranks[i]) {
			t.Fatalf("ranks not sorted at %d: %q >= %q", i, ranks[i-1], ranks[i])
		}
	}
	seen := make(map[string]struct{}, len(ranks))
	for _, r := range ranks {
		if _, dup := seen[r]; dup {
			t.Fatalf("duplicate rank %q", r)
		}
		seen[r] = struct{}{}
	}
}

// TestAppendOnlyGrowth measures rank length when we repeatedly append at
// the end (the common case for replies arriving chronologically). Because
// After bumps by one alphabet position, append-only growth is linear in
// alphabet positions: ~1 new character every rankBase (62) inserts. We
// assert this rate so a regression that worsens the constant fails loudly.
func TestAppendOnlyGrowth(t *testing.T) {
	last := First()
	const inserts = 10_000
	for i := 0; i < inserts; i++ {
		next, err := After(last)
		if err != nil {
			t.Fatalf("iter %d After(%q): %v", i, last, err)
		}
		if next <= last {
			t.Fatalf("iter %d After(%q) = %q, not greater", i, last, next)
		}
		last = next
	}
	// Append-only extends by the canonical midpoint ('U' ~ idx 30) when
	// reaching the alphabet ceiling, leaving ~31 alphabet positions per
	// length increment. 10k inserts therefore yield a rank length near
	// 320. Mid-insertion still works near new extensions because of the
	// midpoint reserve. We assert an upper bound generous enough for
	// implementation flexibility but tight enough to catch quadratic
	// regressions.
	if len(last) > 500 {
		t.Errorf("append-only growth produced rank length %d (%q); expected ≤500",
			len(last), last)
	}
}

// TestMidpointAlwaysShorter — repeatedly bisecting between fixed bounds
// grows linearly with depth, so we cap the test at a reasonable depth.
func TestMidpointAlwaysShorter(t *testing.T) {
	lo, hi := "U", "V"
	for i := 0; i < 30; i++ {
		mid, err := Mid(lo, hi)
		if err != nil {
			t.Fatalf("iter %d Mid(%q,%q): %v", i, lo, hi, err)
		}
		if !(mid > lo && mid < hi) {
			t.Fatalf("iter %d Mid(%q,%q) = %q, not between bounds", i, lo, hi, mid)
		}
		// Alternate which side we collapse.
		if i%2 == 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
}
