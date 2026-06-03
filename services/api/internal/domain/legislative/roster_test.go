package legislative

import "testing"

// Cases use VERBATIM 選挙区 strings from the live 衆議院 議員一覧 (栃木3 / 愛知6 / （比）東海 /
// 比例東京) and a district whose code is cross-checked against the SmartNews/国土数値情報
// GeoJSON property (鳥取2 → kucode 3102). Person names are fictional throughout.
func TestParseShugiinDistrict(t *testing.T) {
	cases := []struct {
		senkyoku string
		wantCode string
		wantName string
		wantPR   bool
		wantBlk  string
		wantOK   bool
	}{
		{"栃木3", "903", "栃木3区", false, "", true},
		{"愛知6", "2306", "愛知6区", false, "", true},
		{"鳥取2", "3102", "鳥取2区", false, "", true}, // == GeoJSON kucode 3102
		{"東京1", "1301", "東京1区", false, "", true},
		{"北海道3", "103", "北海道3区", false, "", true}, // 道 kept in the name
		{"東京１", "1301", "東京1区", false, "", true},  // full-width digit
		{"（比）東海", "", "", true, "東海", true},       // live notation
		{"（比）北関東", "", "", true, "北関東", true},
		{"（比）北海道", "", "", true, "北海道", true}, // block name == prefecture name: must read as PR, not a district
		{"比例東京", "", "", true, "東京", true},    // alternative prefix accepted
		{"（比）", "", "", false, "", false},     // empty block
		{"架空5", "", "", false, "", false},     // unknown prefecture
		{"東京", "", "", false, "", false},      // no district number
		{"", "", "", false, "", false},
	}
	for _, c := range cases {
		code, name, isPR, blk, ok := parseShugiinDistrict(c.senkyoku)
		if ok != c.wantOK || code != c.wantCode || name != c.wantName || isPR != c.wantPR || blk != c.wantBlk {
			t.Errorf("parseShugiinDistrict(%q) = (%q,%q,%v,%q,%v); want (%q,%q,%v,%q,%v)",
				c.senkyoku, code, name, isPR, blk, ok, c.wantCode, c.wantName, c.wantPR, c.wantBlk, c.wantOK)
		}
	}
}

// The geometry-binding invariant mirrors the migration CHECK: a district member has
// a code and no PR block; a 比例 member has a block and no code (never half-bound).
func TestNewShugiinRosterEntryInvariant(t *testing.T) {
	d, ok := NewShugiinRosterEntry("山田 太郎", "やまだ たろう", "自民", "愛知6")
	if !ok {
		t.Fatal("district row should parse")
	}
	if d.House != HouseRepresentatives || d.IsPR || d.DistrictCode == "" || d.PRBlock != "" {
		t.Fatalf("district entry malformed: %+v", d)
	}
	if d.IdentityConfidence != ConfidenceHigh {
		t.Errorf("name+yomi should be high confidence, got %s", d.IdentityConfidence)
	}

	p, ok := NewShugiinRosterEntry("鈴木 一郎", "すずき いちろう", "自民", "比例東京")
	if !ok {
		t.Fatal("PR row should parse")
	}
	if !p.IsPR || p.DistrictCode != "" || p.PRBlock != "東京" {
		t.Fatalf("PR entry malformed: %+v", p)
	}

	if _, ok := NewShugiinRosterEntry("名無し", "ななし", "無", "架空9"); ok {
		t.Error("unknown prefecture should not parse")
	}
}

// 参 roster row shapes from the live 221 名簿 (fictional names): 比例 → 全国区, 合区 → both
// prefecture codes, plain 都道府県 → its JIS code (ADR-000010).
func TestNewSangiinRosterEntry(t *testing.T) {
	pr, ok := NewSangiinRosterEntry("山田　花子", "やまだ　はなこ", "立憲", "比例")
	if !ok || !pr.IsPR || pr.House != HouseCouncillors || pr.PRBlock != "全国" || pr.DistrictCode != "" {
		t.Fatalf("比例 entry malformed: %+v (ok=%v)", pr, ok)
	}

	go2, ok := NewSangiinRosterEntry("鈴木　一郎", "すずき　いちろう", "自民", "鳥取・島根")
	if !ok || go2.IsPR || go2.DistrictCode != "31,32" || go2.DistrictName != "鳥取・島根" {
		t.Fatalf("合区 entry malformed: %+v", go2)
	}

	tk, ok := NewSangiinRosterEntry("佐藤　太郎", "さとう　たろう", "自民", "東京")
	if !ok || tk.DistrictCode != "13" || tk.DistrictName != "東京" || tk.IdentityConfidence != ConfidenceHigh {
		t.Fatalf("都道府県 entry malformed: %+v", tk)
	}

	if _, ok := NewSangiinRosterEntry("名無し", "ななし", "無", "架空県"); ok {
		t.Error("unknown prefecture should not parse")
	}
}

// The roster person_id MUST equal the kokkai-derived id for the same (name, yomi),
// or the read-time vote⨝district join silently misses (ADR-000008 同一導出 discipline).
func TestRosterPersonIDMatchesKokkaiIdentity(t *testing.T) {
	d, ok := NewShugiinRosterEntry("山田 太郎", "やまだ たろう", "立憲", "三重3")
	if !ok {
		t.Fatal("should parse")
	}
	want, _ := PersonIdentity("山田 太郎", "やまだ たろう")
	if d.PersonID != want {
		t.Fatalf("roster person_id %s != kokkai identity %s", d.PersonID, want)
	}
}
