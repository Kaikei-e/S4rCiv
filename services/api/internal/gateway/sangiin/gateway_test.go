package sangiin

import (
	"testing"
)

// Markup shape from the live 参議院 vote-result page (221-0407-v001.htm, UTF-8): title →
// 議案件名, h2.kaiji_nichiji → 回次/日付, h3.tohyosousu → 票数, h4.party → 会派, li.giin →
// pros/cons/names spans. Person names are fictional; bill/party/structure are real.
const votePageHTML = `<html><head><title>令和八年度一般会計予算：本会議投票結果：参議院</title></head><body>
<h2 class="kaiji_nichiji">第221回国会<br>2026年 4月 7日<br>投票結果</h2>
<dl><dt>議案件名</dt><dd>令和八年度一般会計予算</dd></dl>
<h3 class="tohyosousu">投票総数　245<br><span>賛成票　126　　　反対票　119</span></h3>
<h4 class="party">自由民主党・無所属の会(101名)</h4>
<dl class="sanpilist"><dt class="party">賛成票 101　　　反対票   0</dt><dd><ul class="flex">
<li class="giin"><span class="pros">賛成</span><span class="cons"></span><span class="names">山田　　太郎</span></li>
<li class="giin"><span class="pros">賛成</span><span class="cons"></span><span class="names">鈴木　　一郎</span></li>
</ul></dd></dl>
<h4 class="party">立憲民主・社民(38名)</h4>
<dl class="sanpilist"><dt class="party">賛成票 0　　　反対票 38</dt><dd><ul class="flex">
<li class="giin"><span class="pros"></span><span class="cons">反対</span><span class="names">佐藤　　花子</span></li>
</ul></dd></dl>
</body></html>`

func TestParseVotePage(t *testing.T) {
	page, err := New(nil).ParseVotePage([]byte(votePageHTML))
	if err != nil {
		t.Fatal(err)
	}
	if page.Motion != "令和八年度一般会計予算" {
		t.Errorf("motion = %q", page.Motion)
	}
	if page.Session != 221 || page.Date != "2026-04-07" {
		t.Errorf("session/date = %d / %q; want 221 / 2026-04-07", page.Session, page.Date)
	}
	if page.YesCount != 126 || page.NoCount != 119 {
		t.Errorf("tally = %d/%d; want 126/119", page.YesCount, page.NoCount)
	}
	if len(page.Votes) != 3 {
		t.Fatalf("votes = %d; want 3", len(page.Votes))
	}
	if v := page.Votes[0]; v.Name != "山田 太郎" || v.Option != "yes" || v.Group != "自由民主党・無所属の会" {
		t.Errorf("vote[0] = %+v", v)
	}
	if v := page.Votes[2]; v.Name != "佐藤 花子" || v.Option != "no" || v.Group != "立憲民主・社民" {
		t.Errorf("vote[2] = %+v", v)
	}
}
