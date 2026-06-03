package legislative

import (
	"strings"
	"sync"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Voter-name segmentation backed by Kagome (a MeCab-equivalent morphological
// analyzer, pure Go, no cgo). The ipa dictionary is MeCab-IPADIC (ICOT/NAIST
// BSD-style), MIT-packaged — AGPL-compatible; see services/api/THIRD_PARTY_LICENSES.md.
//
// Kagome is used ONLY for free-text 記名投票 rosters, where naive splitting is brittle.
// Structured official fields (e.g. the 選挙区 "岡山1" → kucode) stay on deterministic
// parsers, which are sturdier than morphology on fixed formats. Tokenization is
// deterministic for a fixed dict version, so projection stays reproject-safe — an
// extractor/dict change is a reproject via ExtractorVersion.

var (
	voterTokenizerOnce sync.Once
	voterTokenizer     *tokenizer.Tokenizer
	voterTokenizerErr  error
)

func voterTok() (*tokenizer.Tokenizer, error) {
	voterTokenizerOnce.Do(func() {
		voterTokenizer, voterTokenizerErr = tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	})
	return voterTokenizer, voterTokenizerErr
}

// splitVoterNames segments a 賛成者/反対者 roster block into individual voter names. It
// first splits on explicit separators (spaces/commas/、); then within each run it uses
// Kagome to split at honorific suffix tokens (君/さん tagged 接尾) — robust to OCR that
// runs "逢沢一郎君青木ひとみ君" together with no separator. A 君 that is NOT a 接尾 token
// (i.e. part of a name) does not split, which a naive strings.Split would get wrong.
func splitVoterNames(block string) []string {
	var names []string
	for _, chunk := range nameSplit.Split(block, -1) {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		for _, name := range splitRunOnHonorific(chunk) {
			if n := normalizeName(name); n != "" && !isRoleLabel(n) {
				names = append(names, n)
			}
		}
	}
	return names
}

func splitRunOnHonorific(run string) []string {
	// Fast path: no honorific present (so nothing to split on), or the tokenizer is
	// unavailable → treat the run as a single name.
	if !strings.Contains(run, "君") && !strings.Contains(run, "さん") {
		return []string{run}
	}
	t, err := voterTok()
	if err != nil {
		return []string{run}
	}
	var out []string
	var cur strings.Builder
	for _, tok := range t.Tokenize(run) {
		if isHonorificSuffix(tok) {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteString(tok.Surface)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// isHonorificSuffix reports whether a token is a calling honorific (君/さん) used as a
// 接尾 suffix — i.e. a voter delimiter — and not a 君 that happens to be part of a name.
func isHonorificSuffix(t tokenizer.Token) bool {
	if t.Surface != "君" && t.Surface != "さん" {
		return false
	}
	for _, p := range t.POS() {
		if p == "接尾" {
			return true
		}
	}
	return false
}
