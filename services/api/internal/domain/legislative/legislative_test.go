package legislative

import "testing"

func TestPersonIdentityDeterministicAndConservative(t *testing.T) {
	id1, c1 := PersonIdentity("山田　太郎", "ヤマダ　タロウ")
	id2, c2 := PersonIdentity("山田太郎", "ヤマダタロウ")
	if id1 != id2 {
		t.Fatalf("normalization-equivalent names produced different ids: %s vs %s", id1, id2)
	}
	if c1 != ConfidenceHigh || c2 != ConfidenceHigh {
		t.Fatalf("name+yomi should be high confidence, got %s/%s", c1, c2)
	}

	_, cNoYomi := PersonIdentity("田中花子", "")
	if cNoYomi != ConfidenceMedium {
		t.Fatalf("missing yomi should be medium, got %s", cNoYomi)
	}

	_, cRole := PersonIdentity("議長", "")
	if cRole != ConfidenceLow {
		t.Fatalf("a role label should be low confidence, got %s", cRole)
	}

	// Honorific stripping: the chair's calling appends 君.
	idHon, _ := PersonIdentity("山田太郎君", "ヤマダタロウ")
	if idHon != id1 {
		t.Fatal("trailing 君 should normalize to the same identity")
	}
}

func TestKansujiToInt(t *testing.T) {
	cases := map[string]int{
		"十":     10,
		"百":     100,
		"二百四十二": 242,
		"四百六十五": 465,
		"千":     1000,
		"一万二百":  10200,
	}
	for in, want := range cases {
		got, ok := kansujiToInt(in)
		if !ok || got != want {
			t.Errorf("kansujiToInt(%q) = %d,%v want %d", in, got, ok, want)
		}
	}
	if _, ok := kansujiToInt("abc"); ok {
		t.Error("non-numeral parsed as kansuji")
	}
}

func TestParseNumberTokenMixed(t *testing.T) {
	for _, in := range []string{"242", "２４２"} {
		n, ok := parseNumberToken(in)
		if !ok || n != 242 {
			t.Errorf("parseNumberToken(%q) = %d,%v", in, n, ok)
		}
	}
}

// A clean recorded vote with kanji counts and matching name lists -> high
// confidence, no review needed, correct result.
func TestParseVotesHighConfidence(t *testing.T) {
	text := "本案について記名投票をもって採決いたします。" +
		"投票総数四票、賛成者三、反対者一。よって本案は可決されました。" +
		"賛成者 山田太郎 鈴木一郎 佐藤花子 反対者 田中次郎 以上"
	m := MeetingContent{Speeches: []Speech{{SpeechID: "s1", Text: text}}}

	evs := ParseVotes(m)
	if len(evs) != 1 {
		t.Fatalf("expected 1 vote event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.YesCount != 3 || ev.NoCount != 1 {
		t.Fatalf("counts: yes=%d no=%d", ev.YesCount, ev.NoCount)
	}
	if ev.Result != "passed" {
		t.Fatalf("result = %s, want passed", ev.Result)
	}
	if ev.Confidence != ConfidenceHigh {
		t.Fatalf("confidence = %s, want high", ev.Confidence)
	}
	if ev.NeedsReview {
		t.Fatal("high-confidence event should not need review")
	}
	yes, no := 0, 0
	for _, v := range ev.Votes {
		switch v.Option {
		case "yes":
			yes++
		case "no":
			no++
		}
	}
	if yes != 3 || no != 1 {
		t.Fatalf("extracted votes: yes=%d no=%d", yes, no)
	}
}

// Counts present but name lists missing -> degrade to medium and flag for review.
func TestParseVotesCountsOnlyNeedsReview(t *testing.T) {
	text := "記名投票の結果を報告します。投票総数二百四十二、賛成百三十二、反対百十。"
	m := MeetingContent{Speeches: []Speech{{SpeechID: "s1", Text: text}}}
	evs := ParseVotes(m)
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.YesCount != 132 || ev.NoCount != 110 {
		t.Fatalf("counts: yes=%d no=%d", ev.YesCount, ev.NoCount)
	}
	if ev.Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %s, want medium", ev.Confidence)
	}
	if !ev.NeedsReview {
		t.Fatal("medium-confidence event must need review")
	}
}

func TestParseVotesIgnoresNonVoteSpeech(t *testing.T) {
	m := MeetingContent{Speeches: []Speech{{SpeechID: "s1", Text: "これより会議を開きます。"}}}
	if evs := ParseVotes(m); len(evs) != 0 {
		t.Fatalf("non-vote speech produced %d events", len(evs))
	}
}
