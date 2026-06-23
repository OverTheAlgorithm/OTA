package communitytrend

import "strings"

// RobotsRules holds the Allow/Disallow directives that apply to the generic
// User-agent "*" group. We only ever crawl as a generic bot, so that is the
// only group that matters for our allowance decision.
type RobotsRules struct {
	Allow    []string
	Disallow []string
}

// ParseRobots extracts the "*" group's rules from robots.txt content.
// Records are groups of consecutive User-agent lines followed by rule lines;
// a rule line after a User-agent line begins accumulating into that record.
func ParseRobots(body string) RobotsRules {
	var rules RobotsRules
	var agents []string
	lastWasRule := false

	for _, raw := range strings.Split(body, "\n") {
		line := raw
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colon]))
		val := strings.TrimSpace(line[colon+1:])

		switch key {
		case "user-agent":
			if lastWasRule {
				agents = nil // a new record starts
			}
			agents = append(agents, strings.ToLower(val))
			lastWasRule = false
		case "disallow":
			lastWasRule = true
			if containsAgent(agents, "*") && val != "" {
				rules.Disallow = append(rules.Disallow, val)
			}
		case "allow":
			lastWasRule = true
			if containsAgent(agents, "*") && val != "" {
				rules.Allow = append(rules.Allow, val)
			}
		}
	}
	return rules
}

func containsAgent(agents []string, target string) bool {
	for _, a := range agents {
		if a == target {
			return true
		}
	}
	return false
}

// PathAllowed applies the standard longest-match rule: the most specific
// matching directive wins; ties go to Allow. No matching Disallow => allowed.
func (r RobotsRules) PathAllowed(path string) bool {
	da := longestPrefixMatch(r.Disallow, path)
	if da == "" {
		return true
	}
	aa := longestPrefixMatch(r.Allow, path)
	return len(aa) >= len(da)
}

// AllPathsAllowed reports whether every path is allowed under the "*" group.
func (r RobotsRules) AllPathsAllowed(paths []string) bool {
	for _, p := range paths {
		if !r.PathAllowed(p) {
			return false
		}
	}
	return true
}

// longestPrefixMatch returns the longest directive that is a prefix of path.
func longestPrefixMatch(dirs []string, path string) string {
	best := ""
	for _, d := range dirs {
		if strings.HasPrefix(path, d) && len(d) > len(best) {
			best = d
		}
	}
	return best
}
