package legislative

import "testing"

// lawFixtureXML is a synthetic but structurally valid 法令標準XML. It is the shared
// eId contract fixture: the Rust differ must derive identical eIds over the same
// tree. MainProvision: a Chapter wrapping art 9 (caption, 2 paragraphs, para 1 has
// 1 item) and art 9_2 (枝番, no paragraphs). One SupplProvision with art 3.
const lawFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<Law Era="Heisei" Year="15" LawType="Act" Num="57">
  <LawNum>平成十五年法律第五十七号</LawNum>
  <LawBody>
    <LawTitle Kana="てすとほう">テスト法</LawTitle>
    <MainProvision>
      <Chapter Num="2">
        <ChapterTitle>第二章 総則</ChapterTitle>
        <Article Num="9">
          <ArticleCaption>（目的）</ArticleCaption>
          <ArticleTitle>第九条</ArticleTitle>
          <Paragraph Num="1">
            <ParagraphSentence><Sentence>第一項の本文である。</Sentence></ParagraphSentence>
            <Item Num="2">
              <ItemSentence><Sentence>第二号の本文。</Sentence></ItemSentence>
            </Item>
          </Paragraph>
          <Paragraph Num="2">
            <ParagraphSentence><Sentence>第二項の本文。</Sentence></ParagraphSentence>
          </Paragraph>
        </Article>
        <Article Num="9_2">
          <ArticleCaption>（枝番）</ArticleCaption>
          <ArticleTitle>第九条の二</ArticleTitle>
        </Article>
      </Chapter>
    </MainProvision>
    <SupplProvision>
      <Article Num="3">
        <ArticleTitle>第三条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>附則第三条第一項。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </SupplProvision>
  </LawBody>
</Law>`

func TestParseLawXMLEidContract(t *testing.T) {
	c, err := ParseLawXML([]byte(lawFixtureXML))
	if err != nil {
		t.Fatalf("ParseLawXML: %v", err)
	}

	if c.Law.LawNum != "平成十五年法律第五十七号" {
		t.Fatalf("law_num = %q", c.Law.LawNum)
	}
	if c.Law.Title != "テスト法" || c.Law.TitleKana != "てすとほう" {
		t.Fatalf("title = %q / kana = %q", c.Law.Title, c.Law.TitleKana)
	}

	// Exact materialized tree in document order (the ordinal counter).
	type want struct {
		eid, parent, ntype, num, caption, chapter string
		suppl                                     bool
		sentence                                  string
	}
	wants := []want{
		{eid: "art_9", parent: "", ntype: NodeArticle, num: "9", caption: "（目的）", chapter: "2", suppl: false, sentence: ""},
		{eid: "art_9__para_1", parent: "art_9", ntype: NodeParagraph, num: "1", chapter: "2", suppl: false, sentence: "第一項の本文である。"},
		{eid: "art_9__para_1__item_2", parent: "art_9__para_1", ntype: NodeItem, num: "2", chapter: "2", suppl: false, sentence: "第二号の本文。"},
		{eid: "art_9__para_2", parent: "art_9", ntype: NodeParagraph, num: "2", chapter: "2", suppl: false, sentence: "第二項の本文。"},
		{eid: "art_9_2", parent: "", ntype: NodeArticle, num: "9_2", caption: "（枝番）", chapter: "2", suppl: false, sentence: "第九条の二"},
		{eid: "suppl_1__art_3", parent: "", ntype: NodeArticle, num: "3", caption: "", chapter: "", suppl: true, sentence: ""},
		{eid: "suppl_1__art_3__para_1", parent: "suppl_1__art_3", ntype: NodeParagraph, num: "1", chapter: "", suppl: true, sentence: "附則第三条第一項。"},
	}

	if len(c.Nodes) != len(wants) {
		t.Fatalf("node count = %d, want %d: %+v", len(c.Nodes), len(wants), c.Nodes)
	}
	for i, w := range wants {
		n := c.Nodes[i]
		if n.Ordinal != i {
			t.Errorf("node[%d] ordinal = %d, want %d", i, n.Ordinal, i)
		}
		if n.EID != w.eid {
			t.Errorf("node[%d] eid = %q, want %q", i, n.EID, w.eid)
		}
		if n.ParentEID != w.parent {
			t.Errorf("node[%d] parent_eid = %q, want %q", i, n.ParentEID, w.parent)
		}
		if n.NodeType != w.ntype {
			t.Errorf("node[%d] node_type = %q, want %q", i, n.NodeType, w.ntype)
		}
		if n.Num != w.num {
			t.Errorf("node[%d] num = %q, want %q", i, n.Num, w.num)
		}
		if n.Caption != w.caption {
			t.Errorf("node[%d] caption = %q, want %q", i, n.Caption, w.caption)
		}
		if n.ChapterNum != w.chapter {
			t.Errorf("node[%d] chapter_num = %q, want %q", i, n.ChapterNum, w.chapter)
		}
		if n.IsSuppl != w.suppl {
			t.Errorf("node[%d] is_suppl = %v, want %v", i, n.IsSuppl, w.suppl)
		}
		if n.SentenceText != w.sentence {
			t.Errorf("node[%d] sentence_text = %q, want %q", i, n.SentenceText, w.sentence)
		}
	}
}

func TestLawStreamID(t *testing.T) {
	if got := LawStreamID("415AC0000000057"); got != "egov-law:415AC0000000057" {
		t.Fatalf("LawStreamID = %q", got)
	}
}

// columnSubitemFixtureXML mirrors the real shape of a 定義条 like 第14条: para 1 has an
// item whose sub-clauses sit in <Subitem1>/<Subitem2>, and para 2 carries 用語定義 items
// whose term + 意義 live in two <Column>s. It also has a multi-<Sentence> paragraph to
// pin the 全角スペース join. None of this text was recoverable before ADR-000013.
const columnSubitemFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<Law Era="Heisei" Year="25" LawType="CabinetOrder" Num="280">
  <LawNum>平成二十五年政令第二百八十号</LawNum>
  <LawBody>
    <LawTitle Kana="ていぎてすとれい">定義テスト令</LawTitle>
    <MainProvision>
      <Article Num="14">
        <ArticleCaption>（定義）</ArticleCaption>
        <ArticleTitle>第十四条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence>
            <Sentence>本文である。</Sentence>
            <Sentence>ただし、例外がある。</Sentence>
          </ParagraphSentence>
          <Item Num="2">
            <ItemTitle>二</ItemTitle>
            <ItemSentence><Sentence>次に掲げる額の合算額</Sentence></ItemSentence>
            <Subitem1 Num="1">
              <Subitem1Title>イ</Subitem1Title>
              <Subitem1Sentence><Sentence>イに掲げる額</Sentence></Subitem1Sentence>
            </Subitem1>
            <Subitem1 Num="2">
              <Subitem1Title>ロ</Subitem1Title>
              <Subitem1Sentence><Sentence>ロに掲げる額</Sentence></Subitem1Sentence>
              <Subitem2 Num="1">
                <Subitem2Title>（１）</Subitem2Title>
                <Subitem2Sentence><Sentence>（１）に掲げる率</Sentence></Subitem2Sentence>
              </Subitem2>
            </Subitem1>
          </Item>
        </Paragraph>
        <Paragraph Num="2">
          <ParagraphSentence><Sentence>この条において、次の各号に掲げる用語の意義は、当該各号に定めるところによる。</Sentence></ParagraphSentence>
          <Item Num="1">
            <ItemTitle>一</ItemTitle>
            <ItemSentence>
              <Column><Sentence>みなし計算対象期間</Sentence></Column>
              <Column><Sentence>所定の期間をいう。</Sentence></Column>
            </ItemSentence>
          </Item>
        </Paragraph>
      </Article>
    </MainProvision>
  </LawBody>
</Law>`

func TestParseLawXMLColumnAndSubitem(t *testing.T) {
	c, err := ParseLawXML([]byte(columnSubitemFixtureXML))
	if err != nil {
		t.Fatalf("ParseLawXML: %v", err)
	}

	type want struct {
		eid, parent, ntype, num, sentence string
	}
	wants := []want{
		{eid: "art_14", parent: "", ntype: NodeArticle, num: "14", sentence: ""},
		// 本文＋ただし書 join — the canonical 全角スペース separator.
		{eid: "art_14__para_1", parent: "art_14", ntype: NodeParagraph, num: "1", sentence: "本文である。　ただし、例外がある。"},
		{eid: "art_14__para_1__item_2", parent: "art_14__para_1", ntype: NodeItem, num: "2", sentence: "次に掲げる額の合算額"},
		{eid: "art_14__para_1__item_2__subitem1_1", parent: "art_14__para_1__item_2", ntype: NodeSubitem, num: "1", sentence: "イに掲げる額"},
		{eid: "art_14__para_1__item_2__subitem1_2", parent: "art_14__para_1__item_2", ntype: NodeSubitem, num: "2", sentence: "ロに掲げる額"},
		{eid: "art_14__para_1__item_2__subitem1_2__subitem2_1", parent: "art_14__para_1__item_2__subitem1_2", ntype: NodeSubitem, num: "1", sentence: "（１）に掲げる率"},
		{eid: "art_14__para_2", parent: "art_14", ntype: NodeParagraph, num: "2", sentence: "この条において、次の各号に掲げる用語の意義は、当該各号に定めるところによる。"},
		// 用語定義: term + 意義 across two <Column>s, joined by the 全角スペース.
		{eid: "art_14__para_2__item_1", parent: "art_14__para_2", ntype: NodeItem, num: "1", sentence: "みなし計算対象期間　所定の期間をいう。"},
	}

	if len(c.Nodes) != len(wants) {
		t.Fatalf("node count = %d, want %d: %+v", len(c.Nodes), len(wants), c.Nodes)
	}
	for i, w := range wants {
		n := c.Nodes[i]
		if n.Ordinal != i {
			t.Errorf("node[%d] ordinal = %d, want %d", i, n.Ordinal, i)
		}
		if n.EID != w.eid {
			t.Errorf("node[%d] eid = %q, want %q", i, n.EID, w.eid)
		}
		if n.ParentEID != w.parent {
			t.Errorf("node[%d] parent_eid = %q, want %q", i, n.ParentEID, w.parent)
		}
		if n.NodeType != w.ntype {
			t.Errorf("node[%d] node_type = %q, want %q", i, n.NodeType, w.ntype)
		}
		if n.Num != w.num {
			t.Errorf("node[%d] num = %q, want %q", i, n.Num, w.num)
		}
		if n.SentenceText != w.sentence {
			t.Errorf("node[%d] sentence_text = %q, want %q", i, n.SentenceText, w.sentence)
		}
	}
}
