package legislative

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// Conservative, source-local Popolo identity (ADR-000004 anti-profiling): an id
// is a deterministic function of the observed (name, yomi) within kokkai only.
// No cross-source name resolution, no Wikidata. Same (name, yomi) across meetings
// is treated as the same person; homonyms collide and are flagged via confidence.

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// PersonIdentity returns a stable person_id and an identity confidence.
// Confidence is high when both a name and a reading are present, medium when the
// reading is absent (homonym risk), low when the name is empty or looks like a
// procedural role rather than a person.
func PersonIdentity(name, yomi string) (id, confidence string) {
	n := normalizeName(name)
	y := normalizeReading(yomi)
	id = "p:" + shortHash(n+"\n"+y)
	switch {
	case n == "" || isRoleLabel(n):
		confidence = ConfidenceLow
	case y == "":
		confidence = ConfidenceMedium
	default:
		confidence = ConfidenceHigh
	}
	return id, confidence
}

// OrganizationIdentity returns a stable org_id for a 会派/政党 name.
func OrganizationIdentity(name string) string {
	return "o:" + shortHash(normalizeName(name))
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// normalizeName trims ASCII and full-width spaces and strips trailing transcript
// honorifics (君 / さん) that the chair's calling appends.
func normalizeName(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "　", " "))
	s = strings.Join(strings.Fields(s), "")
	for _, suf := range []string{"君", "さん"} {
		s = strings.TrimSuffix(s, suf)
	}
	return s
}

func normalizeReading(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "　", " ")), "")
}

// isRoleLabel flags speaker strings that are procedural roles, not people.
func isRoleLabel(n string) bool {
	return slices.Contains([]string{"議長", "副議長", "委員長", "主査", "会長"}, n)
}
