//! Contract tests over synthetic but structurally valid 法令標準XML.
//!
//! These pin the SHARED eId derivation (ADR-000005) and the diff/classification
//! behavior against the public `xmlmodel::parse` + `diff::compute` API. If an eId
//! here changes, the Go projector must change in lockstep.

use differ::diff::{self, ChangeOp, Classification, Confidence};
use differ::xmlmodel::{self, NodeType};

/// A small law: MainProvision with two articles (article 9 has two paragraphs and
/// an item under paragraph 1), plus one SupplProvision article.
fn base_law() -> &'static str {
    r#"<?xml version="1.0" encoding="UTF-8"?>
<Law>
  <LawBody>
    <MainProvision>
      <Article Num="8">
        <ArticleCaption>（定義）</ArticleCaption>
        <ArticleTitle>第八条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>この法律において用語の意義は次のとおりとする。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
      <Article Num="9">
        <ArticleCaption>（適用範囲）</ArticleCaption>
        <ArticleTitle>第九条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>この法律は全ての事業者に適用する。</Sentence></ParagraphSentence>
          <Item Num="1">
            <ItemSentence><Sentence>国内に住所を有する者</Sentence></ItemSentence>
          </Item>
        </Paragraph>
        <Paragraph Num="2">
          <ParagraphSentence><Sentence>前項の規定は外国法人にも準用する。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </MainProvision>
    <SupplProvision>
      <Article Num="1">
        <ArticleCaption>（施行期日）</ArticleCaption>
        <ArticleTitle>第一条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>この法律は公布の日から施行する。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </SupplProvision>
  </LawBody>
</Law>"#
}

#[test]
fn eid_set_and_node_types_match_contract() {
    let law = xmlmodel::parse(base_law().as_bytes()).expect("parse base law");

    let mut eids: Vec<&str> = law.nodes.keys().map(String::as_str).collect();
    eids.sort_unstable();

    let mut expected = vec![
        "art_8",
        "art_8__para_1",
        "art_9",
        "art_9__para_1",
        "art_9__para_1__item_1",
        "art_9__para_2",
        "suppl_1__art_1",
        "suppl_1__art_1__para_1",
    ];
    expected.sort_unstable();

    assert_eq!(eids, expected, "eId set must match the shared contract");

    // node_type checks
    assert_eq!(law.nodes["art_9"].node_type, NodeType::Article);
    assert_eq!(law.nodes["art_9__para_1"].node_type, NodeType::Paragraph);
    assert_eq!(law.nodes["art_9__para_1__item_1"].node_type, NodeType::Item);
    assert_eq!(law.nodes["suppl_1__art_1"].node_type, NodeType::Article);

    // num tokens
    assert_eq!(law.nodes["art_9"].num, "9");
    assert_eq!(law.nodes["art_9__para_2"].num, "2");
    assert_eq!(law.nodes["art_9__para_1__item_1"].num, "1");
}

#[test]
fn sentence_text_is_node_owned() {
    let law = xmlmodel::parse(base_law().as_bytes()).expect("parse base law");

    assert_eq!(
        law.nodes["art_9__para_1"].sentence_text,
        "この法律は全ての事業者に適用する。"
    );
    assert_eq!(
        law.nodes["art_9__para_2"].sentence_text,
        "前項の規定は外国法人にも準用する。"
    );
    assert_eq!(
        law.nodes["art_9__para_1__item_1"].sentence_text,
        "国内に住所を有する者"
    );
    // An article that has paragraphs owns no direct sentence text.
    assert_eq!(law.nodes["art_9"].sentence_text, "");
    // The caption is captured separately, and the ArticleTitle is excluded.
    assert_eq!(law.nodes["art_9"].caption, "（適用範囲）");

    // Suppl-prefixed node text
    assert_eq!(
        law.nodes["suppl_1__art_1__para_1"].sentence_text,
        "この法律は公布の日から施行する。"
    );
}

#[test]
fn eda_ban_num_flows_into_eid_verbatim() {
    // 第9条の2 is encoded as Num="9_2" and must appear verbatim in the eId.
    let xml = r#"<Law><LawBody><MainProvision>
      <Article Num="9_2">
        <ArticleTitle>第九条の二</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>枝番条文。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </MainProvision></LawBody></Law>"#;
    let law = xmlmodel::parse(xml.as_bytes()).expect("parse");
    assert!(law.nodes.contains_key("art_9_2"));
    assert!(law.nodes.contains_key("art_9_2__para_1"));
    assert_eq!(law.nodes["art_9_2"].num, "9_2");
}

#[test]
fn identical_snapshots_yield_no_changes_administrative() {
    let prev = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let curr = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let result = diff::compute(&prev, &curr);

    assert!(result.changes.is_empty());
    assert_eq!(result.classification, Classification::Administrative);
    assert_eq!(result.confidence, Confidence::High);
}

#[test]
fn modifying_a_paragraph_sentence_is_substantive() {
    let modified = base_law().replace(
        "前項の規定は外国法人にも準用する。",
        "前項の規定は外国法人には適用しない。",
    );
    let prev = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let curr = xmlmodel::parse(modified.as_bytes()).unwrap();
    let result = diff::compute(&prev, &curr);

    assert_eq!(result.changes.len(), 1);
    let change = &result.changes[0];
    assert_eq!(change.eid, "art_9__para_2");
    assert_eq!(change.op, ChangeOp::Modified);
    assert_eq!(change.node_type, NodeType::Paragraph);
    assert_eq!(change.prev_text, "前項の規定は外国法人にも準用する。");
    assert_eq!(change.curr_text, "前項の規定は外国法人には適用しない。");

    assert_eq!(result.classification, Classification::Substantive);
    assert_eq!(result.confidence, Confidence::High);
}

#[test]
fn adding_an_article_is_substantive() {
    let added = base_law().replace(
        "    </MainProvision>",
        r#"      <Article Num="10">
        <ArticleCaption>（罰則）</ArticleCaption>
        <ArticleTitle>第十条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>違反した者は罰金に処する。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </MainProvision>"#,
    );
    let prev = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let curr = xmlmodel::parse(added.as_bytes()).unwrap();
    let result = diff::compute(&prev, &curr);

    // The new article node has empty text (paragraphs hold it), but its paragraph
    // is an ADDED text-bearing node → substantive.
    let added_para = result
        .changes
        .iter()
        .find(|c| c.eid == "art_10__para_1")
        .expect("art_10__para_1 should be ADDED");
    assert_eq!(added_para.op, ChangeOp::Added);
    assert_eq!(added_para.curr_text, "違反した者は罰金に処する。");
    assert!(added_para.prev_text.is_empty());

    assert!(result.changes.iter().any(|c| c.eid == "art_10"));
    assert_eq!(result.classification, Classification::Substantive);
    assert_eq!(result.confidence, Confidence::High);
}

#[test]
fn caption_only_change_is_administrative() {
    let recaptioned = base_law().replace("（適用範囲）", "（この法律の適用範囲）");
    let prev = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let curr = xmlmodel::parse(recaptioned.as_bytes()).unwrap();
    let result = diff::compute(&prev, &curr);

    // Caption-only edits produce no NodeChange and stay administrative.
    assert!(
        result.changes.is_empty(),
        "caption-only change must not emit a normative NodeChange, got {:?}",
        result.changes
    );
    assert_eq!(result.classification, Classification::Administrative);
    assert_eq!(result.confidence, Confidence::High);
}

#[test]
fn deleting_an_article_is_substantive() {
    // Drop article 8 entirely.
    let deleted = base_law().replace(
        r#"      <Article Num="8">
        <ArticleCaption>（定義）</ArticleCaption>
        <ArticleTitle>第八条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>この法律において用語の意義は次のとおりとする。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
"#,
        "",
    );
    let prev = xmlmodel::parse(base_law().as_bytes()).unwrap();
    let curr = xmlmodel::parse(deleted.as_bytes()).unwrap();
    let result = diff::compute(&prev, &curr);

    let del_para = result
        .changes
        .iter()
        .find(|c| c.eid == "art_8__para_1")
        .expect("art_8__para_1 should be DELETED");
    assert_eq!(del_para.op, ChangeOp::Deleted);
    assert_eq!(
        del_para.prev_text,
        "この法律において用語の意義は次のとおりとする。"
    );
    assert_eq!(result.classification, Classification::Substantive);
}
