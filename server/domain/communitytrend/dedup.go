package communitytrend

import (
	"crypto/sha256"
	"encoding/hex"
)

// Fingerprint produces a one-way hash identifying a source item within a
// community. Stored in ct_seen_posts so the original id/title need not be kept
// (copyright guardrail: the fingerprint cannot be reversed to content).
func Fingerprint(communityKey, sourceID string) string {
	sum := sha256.Sum256([]byte(communityKey + "\x00" + sourceID))
	return hex.EncodeToString(sum[:])
}

// FilterUnseen splits items into those whose fingerprint is NOT in seen
// (the fresh items to count, per the new-inflow counting model, decisions.md D-001)
// and returns the fingerprints of those fresh items for persistence.
func FilterUnseen(communityKey string, items []TrendItem, seen map[string]bool) (fresh []TrendItem, freshFingerprints []string) {
	for _, it := range items {
		fp := Fingerprint(communityKey, it.SourceID)
		if seen[fp] {
			continue
		}
		fresh = append(fresh, it)
		freshFingerprints = append(freshFingerprints, fp)
	}
	return fresh, freshFingerprints
}
