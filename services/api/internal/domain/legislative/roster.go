package legislative

import (
	"strconv"
	"strings"
)

// Roster: the legislator -> electoral-district binding projected from the
// giin-roster (両院公式議員名簿) source (ADR-000008). Pure and deterministic, so the
// projection is reproject-safe. PersonID reuses the same conservative Popolo
// identity as kokkai (PersonIdentity), so a roster entry joins a 記名投票 voter by
// value — the read-time join that colours the district vote map.
//
// Legislators are accountable public officials, so this geographic binding is NOT
// the private-person profiling DISCIPLINE §4 forbids (ADR-000006).

const (
	HouseRepresentatives = "衆議院" // 衆議院
	HouseCouncillors     = "参議院" // 参議院
)

// RosterStreamID is the deterministic stream identity for one giin-roster page
// (houseKey like "shugiin-a"), mirroring MeetingStreamID / LawStreamID.
func RosterStreamID(pageKey string) string { return "giin-roster:" + pageKey }

// NameKey is the normalized join key for matching a vote's voter name to a roster
// member by name (used for 参 votes, which carry no ふりがな so person_id — which bakes
// in yomi — can't match the roster). Same normalization as PersonIdentity's name half.
func NameKey(name string) string { return normalizeName(name) }

// RosterEntry is one current member's seat. The geometry-binding invariant matches
// the migration CHECK: a district member has DistrictCode (== GeoJSON kucode); a
// 比例 member has none and is shown in the companion panel, never erased (§5).
type RosterEntry struct {
	PersonID           string
	NameKey            string // normalized name, for the 参 vote→roster name join
	Name               string
	Yomi               string
	House              string // 衆議院 | 参議院
	DistrictCode       string // strconv(ken*100+ku) == 国土数値情報/SmartNews kucode; "" when IsPR
	DistrictName       string // 人間可読 (栃木3区); "" when IsPR
	IsPR               bool   // 比例選出
	PRBlock            string // 比例ブロック (衆: 東海/東京…, 参: 全国); "" when not PR
	ParliamentaryGroup string // 会派
	IdentityConfidence string
}

// NewShugiinRosterEntry builds a 衆議院 roster entry from one 議員一覧 row
// (氏名 / ふりがな / 会派 / 選挙区). Returns ok=false when 選挙区 cannot be parsed.
func NewShugiinRosterEntry(name, yomi, group, senkyoku string) (RosterEntry, bool) {
	code, districtName, isPR, block, ok := parseShugiinDistrict(senkyoku)
	if !ok {
		return RosterEntry{}, false
	}
	pid, conf := PersonIdentity(name, yomi)
	return RosterEntry{
		PersonID:           pid,
		NameKey:            NameKey(name),
		Name:               displayName(name),
		Yomi:               collapseSpaces(yomi),
		House:              HouseRepresentatives,
		DistrictCode:       code,
		DistrictName:       districtName,
		IsPR:               isPR,
		PRBlock:            block,
		ParliamentaryGroup: strings.TrimSpace(group),
		IdentityConfidence: conf,
	}, true
}

// NewSangiinRosterEntry builds a 参議院 roster entry from one 議員一覧 row
// (議員氏名 / 読み方 / 会派 / 選挙区). 選挙区 is a 都道府県 name, a 合区 ("鳥取・島根"), or
// "比例" (全国区, ADR-000010). For the 参 map, district_code is the JIS prefecture
// code(s) (NOT the 衆 ken*100+ku kucode); the house field distinguishes the two
// schemes. 合区 carries both codes, comma-joined, so the map can colour both.
func NewSangiinRosterEntry(name, yomi, group, senkyoku string) (RosterEntry, bool) {
	pid, conf := PersonIdentity(name, yomi)
	e := RosterEntry{
		PersonID:           pid,
		NameKey:            NameKey(name),
		Name:               displayName(name),
		Yomi:               collapseSpaces(yomi),
		House:              HouseCouncillors,
		ParliamentaryGroup: strings.TrimSpace(group),
		IdentityConfidence: conf,
	}
	s := strings.TrimSpace(senkyoku)
	if s == "比例" || s == "比例代表" {
		e.IsPR = true
		e.PRBlock = "全国"
		return e, true
	}
	var codes []string
	for _, p := range strings.Split(s, "・") { // 合区 splits on ・
		c, ok := prefectureCode[strings.TrimSpace(p)]
		if !ok {
			return RosterEntry{}, false
		}
		codes = append(codes, strconv.Itoa(c))
	}
	if len(codes) == 0 {
		return RosterEntry{}, false
	}
	e.DistrictCode = strings.Join(codes, ",")
	e.DistrictName = s
	return e, true
}

// parseShugiinDistrict normalizes the 衆議院 選挙区 cell. Single-member districts are
// written "<県名><番号>" with no 都/道/府/県 or 区 suffix (e.g. "栃木3", "鳥取2"); 比例
// is "比例<ブロック>" (e.g. "比例東海"). The district code is ken*100+番号 so it equals
// the SmartNews/国土数値情報 GeoJSON `kucode` property — the shared "同一導出" join key.
func parseShugiinDistrict(senkyoku string) (code, name string, isPR bool, block string, ok bool) {
	s := strings.TrimSpace(normalizeFullwidthDigits(senkyoku))
	// 比例 is written "（比）<ブロック>" on the live 衆 roster (e.g. （比）北関東); also
	// accept "比例<ブロック>". This MUST run before the district parse because a block
	// name can equal a prefecture name (（比）北海道).
	for _, prefix := range []string{"（比）", "比例"} {
		if rest, found := strings.CutPrefix(s, prefix); found {
			block = strings.TrimSpace(rest)
			if block == "" {
				return "", "", false, "", false
			}
			return "", "", true, block, true
		}
	}
	// Split the trailing ASCII digits (the district number) from the prefecture name.
	// UTF-8 continuation bytes are >= 0x80, so a byte-wise ASCII-digit scan is safe.
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	pref, numStr := s[:i], s[i:]
	if pref == "" || numStr == "" {
		return "", "", false, "", false
	}
	pc, known := prefectureCode[pref]
	if !known {
		return "", "", false, "", false
	}
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		return "", "", false, "", false
	}
	return strconv.Itoa(pc*100 + num), pref + numStr + "区", false, "", true
}

// displayName cleans a 名簿 name cell for display: full-width spaces to a single
// ASCII space and the calling honorific (君/さん) stripped (e.g. "逢沢　　一郎君" →
// "逢沢 一郎"). Identity still flows through PersonIdentity's own normalization.
func displayName(s string) string {
	s = collapseSpaces(s)
	for _, suf := range []string{"君", "さん"} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimSpace(s)
}

// collapseSpaces trims and collapses any run of (incl. full-width) whitespace to a
// single ASCII space.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "　", " ")), " ")
}

// normalizeFullwidthDigits maps full-width ０-９ to ASCII 0-9 within a mixed string,
// leaving non-digit runes intact (the 名簿 may use either digit form). Distinct from
// the all-digit toASCIIDigits in numbers.go.
func normalizeFullwidthDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + (r - '０')
		}
		return r
	}, s)
}

// prefectureCode maps the short prefecture name as it appears in the 衆議院 名簿
// (no 都/府/県; 北海道 keeps 道) to its JIS X 0401 code (== GeoJSON `ken`).
var prefectureCode = map[string]int{
	"北海道": 1, "青森": 2, "岩手": 3, "宮城": 4, "秋田": 5, "山形": 6, "福島": 7,
	"茨城": 8, "栃木": 9, "群馬": 10, "埼玉": 11, "千葉": 12, "東京": 13, "神奈川": 14,
	"新潟": 15, "富山": 16, "石川": 17, "福井": 18, "山梨": 19, "長野": 20, "岐阜": 21,
	"静岡": 22, "愛知": 23, "三重": 24, "滋賀": 25, "京都": 26, "大阪": 27, "兵庫": 28,
	"奈良": 29, "和歌山": 30, "鳥取": 31, "島根": 32, "岡山": 33, "広島": 34, "山口": 35,
	"徳島": 36, "香川": 37, "愛媛": 38, "高知": 39, "福岡": 40, "佐賀": 41, "長崎": 42,
	"熊本": 43, "大分": 44, "宮崎": 45, "鹿児島": 46, "沖縄": 47,
}
