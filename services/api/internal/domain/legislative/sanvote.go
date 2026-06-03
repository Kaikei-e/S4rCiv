package legislative

import (
	"strconv"
	"strings"
)

// 参議院本会議投票結果 (touhyoulist) — per-member named votes (ADR-000010). The kokkai
// 会議録 records 衆 記名投票 as counts only, so the district vote map sources per-member
// votes from the 参議院 roll-call pages, which publish each member's 賛否 by name.

// SangiinNamedVote is one member's recorded vote on a 参議院本会議 議案.
type SangiinNamedVote struct {
	Name   string // 議員名 (as printed; normalize for joins via normalizeName)
	Option string // yes | no | abstain
	Group  string // 会派
}

// SangiinVotePage is one 参議院 roll-call (one 議案), parsed from a vote-result page.
type SangiinVotePage struct {
	Session  int    // 国会回次
	Motion   string // 議案件名
	Date     string // YYYY-MM-DD
	YesCount int    // 賛成票 (announced total)
	NoCount  int    // 反対票 (announced total)
	Votes    []SangiinNamedVote
}

// SangiinOption maps the pros/cons span texts of one <li class="giin"> to a vote
// option. 賛成 → yes, 反対 → no, otherwise (欠席/投票なし) → abstain.
func SangiinOption(pros, cons string) string {
	switch {
	case strings.Contains(pros, "賛成"):
		return "yes"
	case strings.Contains(cons, "反対"):
		return "no"
	default:
		return "abstain"
	}
}

// CleanParliamentaryGroup strips the trailing "(NN名)" headcount from a 会派 heading
// (e.g. "自由民主党・無所属の会(101名)" → "自由民主党・無所属の会").
func CleanParliamentaryGroup(s string) string {
	s = collapseSpaces(s)
	if i := strings.LastIndexByte(s, '('); i > 0 && strings.HasSuffix(s, ")") {
		s = s[:i]
	}
	// also tolerate full-width parens
	if i := strings.LastIndex(s, "（"); i > 0 && strings.HasSuffix(s, "）") {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// ParseSangiinHeadingNumber pulls the first ASCII/full-width integer out of a string
// like "投票総数　245" or "賛成票　126" (returns 0 if none).
func ParseSangiinHeadingNumber(s string) int {
	digits := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r >= '０' && r <= '９':
			return '0' + (r - '０')
		default:
			return -1
		}
	}, s)
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}
