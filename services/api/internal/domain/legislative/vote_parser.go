package legislative

import (
	"regexp"
	"strings"
)

// ExtractorVersion stamps every projected vote so re-parsing with an improved
// extractor is a reproject, not an in-place mutation (immutable-design-guard #6).
const ExtractorVersion = "kokkai-vote/0.1.0"

// Deterministic, no-LLM 記名投票 extraction (ADR/DISCIPLINE §6). The transcript
// tail is free text with OCR noise, so this is best-effort and confidence-bearing:
// anything below "high" sets NeedsReview and is queued for a human verdict, which
// is recorded as a Tier 1 interpretation.event and folded back on reproject.

type ParsedVote struct {
	Option    string // "yes" | "no" | "abstain"
	VoterName string // raw extracted name (not yet resolved to a Person)
}

type ParsedVoteEvent struct {
	Motion         string
	YesCount       int
	NoCount        int
	AbstainCount   int
	Result         string // "passed" | "rejected" | "unknown"
	Confidence     string
	NeedsReview    bool
	Votes          []ParsedVote
	SourceSpeechID string
}

var (
	reTotal   = regexp.MustCompile(`投票総数[^0-9０-９〇零一二三四五六七八九十百千万]*([0-9０-９〇零一二三四五六七八九十百千万]+)`)
	reYes     = regexp.MustCompile(`賛成[者数]?[^0-9０-９〇零一二三四五六七八九十百千万]*([0-9０-９〇零一二三四五六七八九十百千万]+)`)
	reNo      = regexp.MustCompile(`反対[者数]?[^0-9０-９〇零一二三四五六七八九十百千万]*([0-9０-９〇零一二三四五六七八九十百千万]+)`)
	reMotion  = regexp.MustCompile(`([^。\s]{2,40}?)(?:案件?|の件)(?:について|の採決)`)
	nameSplit = regexp.MustCompile(`[\s　、,，]+`)
)

// ParseVotes scans a meeting's speeches and returns one event per recorded vote.
func ParseVotes(m MeetingContent) []ParsedVoteEvent {
	var out []ParsedVoteEvent
	for _, sp := range m.Speeches {
		if !looksLikeRecordedVote(sp.Text) {
			continue
		}
		out = append(out, parseOne(sp))
	}
	return out
}

func looksLikeRecordedVote(text string) bool {
	if strings.Contains(text, "記名投票") {
		return true
	}
	return strings.Contains(text, "投票総数") && strings.Contains(text, "賛成") && strings.Contains(text, "反対")
}

func parseOne(sp Speech) ParsedVoteEvent {
	compact := stripSpaces(sp.Text)

	yes, yesOK := matchCount(reYes, compact)
	no, noOK := matchCount(reNo, compact)
	total, totalOK := matchCount(reTotal, compact)

	ev := ParsedVoteEvent{
		YesCount:       yes,
		NoCount:        no,
		Motion:         extractMotion(sp.Text),
		SourceSpeechID: sp.SpeechID,
	}

	yesNames := extractNameBlock(sp.Text, "賛成", []string{"反対", "投票総数", "以上"})
	noNames := extractNameBlock(sp.Text, "反対", []string{"賛成", "投票総数", "以上"})
	for _, n := range yesNames {
		ev.Votes = append(ev.Votes, ParsedVote{Option: "yes", VoterName: n})
	}
	for _, n := range noNames {
		ev.Votes = append(ev.Votes, ParsedVote{Option: "no", VoterName: n})
	}

	ev.Result = result(yes, no, yesOK && noOK)
	ev.Confidence = confidence(yesOK, noOK, totalOK, total, yes, no, len(yesNames), len(noNames))
	ev.NeedsReview = ev.Confidence != ConfidenceHigh
	return ev
}

func result(yes, no int, counted bool) string {
	switch {
	case !counted:
		return "unknown"
	case yes > no:
		return "passed"
	default:
		return "rejected"
	}
}

// confidence is high only when counts parse AND the totals are internally
// consistent AND the extracted name lists match the announced counts; otherwise
// it degrades, sending the event to human review.
func confidence(yesOK, noOK, totalOK bool, total, yes, no, nYes, nNo int) string {
	if !yesOK || !noOK {
		return ConfidenceLow
	}
	countsConsistent := !totalOK || total == yes+no
	namesMatch := nYes == yes && nNo == no
	switch {
	case countsConsistent && namesMatch:
		return ConfidenceHigh
	case countsConsistent:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func matchCount(re *regexp.Regexp, s string) (int, bool) {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	return parseNumberToken(m[1])
}

func extractMotion(text string) string {
	m := reMotion.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// extractNameBlock returns the names between a start marker and the first of the
// stop markers. Best-effort: noisy blocks yield names that won't match counts,
// which the confidence rule then flags.
func extractNameBlock(text, start string, stops []string) []string {
	// The name roster follows the count summary, so the last "<start>者" marker
	// is the roster (an earlier one, if any, is "<start>者<count>"). Heuristic:
	// imperfect rosters simply fail the count match and degrade confidence.
	i := strings.LastIndex(text, start+"者")
	if i < 0 {
		return nil
	}
	rest := text[i+len(start+"者"):]
	end := len(rest)
	for _, stop := range stops {
		if j := strings.Index(rest, stop); j >= 0 && j < end {
			end = j
		}
	}
	block := rest[:end]
	var names []string
	for _, n := range nameSplit.Split(block, -1) {
		n = normalizeName(n)
		if n != "" && !isRoleLabel(n) {
			names = append(names, n)
		}
	}
	return names
}

func stripSpaces(s string) string {
	return strings.NewReplacer(" ", "", "　", "", "\n", "", "\t", "").Replace(s)
}
