package giinroster

import (
	"testing"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// rosterHTML is copied VERBATIM from the live 衆議院 議員一覧 (1giin.htm), decoded to
// UTF-8: the header row, one 小選挙区 member (逢沢一郎 / 岡山1) and one 比例 member
// (青木ひとみ / （比）北関東). Tags are uppercase, cells nest <TT>/<CENTER>/<a> with
// embedded newlines and full-width spaces — the parser must survive all of it.
const rosterHTML = `<html><body><table>
<TR>
<TD class="sh1td1"><TT class="sh1tt1"><center><B>氏名</B></center></TT></TD>
<TD class="sh1td2"><TT class="sh1tt1"><center><B>ふりがな</B></center></TT></TD>
<TD class="sh1td3"><TT class="sh1tt1"><center><B>会派</B></center></TT></TD>
<TD class="sh1td1"><TT class="sh1tt1"><center><B>選挙区</B></center></TT></TD>
<TD class="sh1td4"><TT class="sh1tt1"><center><B>当選回数</B></center></TT></TD>
</TR>
<TR VALIGN = top><TD class="sh1td5"><TT class="sh1tt1"><a
href='../../../../itdb_giinprof.nsf/html/profile/001.html'>逢沢　　一郎君</a>
</TT></TD>
<TD class="sh1td6"><TT class="sh1tt1">あいさわ
　いちろう
</TT></TD>
<TD class="sh1td7"><TT class="sh1tt1"><CENTER>自民
</CENTER></TT></TD>
<TD class="sh1td5"><TT class="sh1tt1">岡山1
</TT></TD>
<TD class="sh1td8"><TT class="sh1tt1"><CENTER>14
</CENTER></TT></TD>
</TR>
<TR VALIGN = top><TD class="sh1td5"><TT class="sh1tt1"><a
href='../../../../itdb_giinprof.nsf/html/profile/002.html'>青木　ひとみ君</a>
</TT></TD>
<TD class="sh1td6"><TT class="sh1tt1">あおき
　ひとみ
</TT></TD>
<TD class="sh1td7"><TT class="sh1tt1"><CENTER>参政
</CENTER></TT></TD>
<TD class="sh1td5"><TT class="sh1tt1">（比）北関東
</TT></TD>
<TD class="sh1td8"><TT class="sh1tt1"><CENTER>1
</CENTER></TT></TD>
</TR>
</table></body></html>`

func TestParseRoster(t *testing.T) {
	g := New(nil)
	got, err := g.ParseRoster([]byte(rosterHTML))
	if err != nil {
		t.Fatalf("ParseRoster: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 members (header + nav skipped), got %d: %+v", len(got), got)
	}

	// 小選挙区 member: 岡山1 → ken 33 → kucode 3301, district name 岡山1区, not PR.
	d := got[0]
	if d.Name != "逢沢 一郎" || d.House != leg.HouseRepresentatives || d.IsPR ||
		d.DistrictCode != "3301" || d.DistrictName != "岡山1区" || d.ParliamentaryGroup != "自民" {
		t.Errorf("district member mismatch: %+v", d)
	}
	if want, _ := leg.PersonIdentity("逢沢　　一郎君", "あいさわ いちろう"); d.PersonID != want {
		t.Errorf("person_id %s != kokkai identity %s (join would miss)", d.PersonID, want)
	}

	// 比例 member: （比）北関東 → PR, block 北関東, no district code.
	p := got[1]
	if p.Name != "青木 ひとみ" || !p.IsPR || p.PRBlock != "北関東" || p.DistrictCode != "" ||
		p.ParliamentaryGroup != "参政" {
		t.Errorf("PR member mismatch: %+v", p)
	}
}
