package sangiin

import (
	"testing"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// Table shape from the live 参 221 名簿: 7-th header row, then member rows of 6 <td>
// (議員氏名/読み方/会派/選挙区/任期/blank); a 五十音 group's first row prefixes a <th>.
// Person names fictional; 選挙区/会派/structure real.
const rosterHTML = `<table>
<tr><th scope="col">&nbsp;</th><th scope="col">議員氏名</th><th scope="col">読み方</th><th scope="col">会派</th><th scope="col">選挙区</th><th scope="col">任期満了</th><th scope="col">&nbsp;</th></tr>
<tr><th scope="row" rowspan="2"><b>や</b></th>
<td><a href="../profile/1.htm">山田　　花子</a></td><td>やまだ　はなこ</td><td>立憲</td><td>比例</td><td>令和10年7月25日</td><td>&nbsp;</td></tr>
<tr><td><a href="../profile/2.htm">鈴木　　一郎</a></td><td>すずき　いちろう</td><td>自民</td><td>鳥取・島根</td><td>令和10年7月25日</td><td>&nbsp;</td></tr>
</table>`

func TestParseRoster(t *testing.T) {
	got, err := New(nil).ParseRoster([]byte(rosterHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 members (header skipped), got %d: %+v", len(got), got)
	}

	// 比例 member → 全国区 PR, no district code.
	if v := got[0]; v.Name != "山田 花子" || v.House != leg.HouseCouncillors || !v.IsPR ||
		v.DistrictCode != "" || v.PRBlock != "全国" || v.ParliamentaryGroup != "立憲" {
		t.Errorf("PR member mismatch: %+v", v)
	}
	// 合区 member → both prefecture codes.
	if v := got[1]; v.Name != "鈴木 一郎" || v.IsPR || v.DistrictCode != "31,32" ||
		v.DistrictName != "鳥取・島根" || v.ParliamentaryGroup != "自民" {
		t.Errorf("合区 member mismatch: %+v", v)
	}
}
