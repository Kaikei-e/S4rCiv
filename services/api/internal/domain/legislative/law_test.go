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
