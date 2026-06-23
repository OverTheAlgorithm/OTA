package communitytrend

import "testing"

func TestParseRobots_StarGroup(t *testing.T) {
	body := `
# comment
User-agent: Googlebot
Disallow: /private

User-agent: *
Allow: /service/board/
Disallow: /service/
Disallow: /*?*

User-agent: BadBot
Disallow: /
`
	r := ParseRobots(body)
	// only the "*" group should be captured
	if len(r.Disallow) != 2 {
		t.Fatalf("expected 2 disallow for *, got %v", r.Disallow)
	}
	if len(r.Allow) != 1 || r.Allow[0] != "/service/board/" {
		t.Fatalf("expected 1 allow /service/board/, got %v", r.Allow)
	}
}

func TestRobots_PathAllowed(t *testing.T) {
	r := RobotsRules{
		Allow:    []string{"/service/board/"},
		Disallow: []string{"/service/"},
	}
	tests := []struct {
		path string
		want bool
	}{
		{"/service/board/list", true}, // allow more specific than disallow
		{"/service/mypage", false},    // only disallow matches
		{"/about", true},              // nothing matches
		{"/service/", false},          // exact disallow
	}
	for _, tc := range tests {
		if got := r.PathAllowed(tc.path); got != tc.want {
			t.Fatalf("PathAllowed(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}

func TestRobots_EmptyMeansAllow(t *testing.T) {
	// dogdrip-style: "Allow: /" with crawl-delay; no real disallow of board
	r := ParseRobots("User-agent: *\nAllow: /\nCrawl-delay: 10\n")
	if !r.AllPathsAllowed([]string{"/boomupbest"}) {
		t.Fatal("expected /boomupbest allowed when only Allow: / present")
	}
}

func TestRobots_NoStarGroupAllowsAll(t *testing.T) {
	// robots that only restricts named bots leaves "*" unrestricted
	r := ParseRobots("User-agent: BadBot\nDisallow: /\n")
	if !r.AllPathsAllowed([]string{"/anything"}) {
		t.Fatal("expected all allowed when no * group disallows")
	}
}
