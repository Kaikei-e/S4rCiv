//! Parse 法令標準XML (e-Gov law standard XML) into a flat map of normative nodes
//! keyed by AKN eId.
//!
//! eId derivation is a SHARED CONTRACT with the Go projector (ADR-000005): the
//! differ's output nodes must join byte-for-byte to the Go-side `law_node` rows.
//! The rules implemented here are:
//!
//! - Article   → `art_{Num}`                       (Num verbatim, `_` for 枝番 e.g. `9_2`)
//! - Paragraph → `{article_eid}__para_{Num}`        (Num defaults to 1-based ordinal)
//! - Item      → `{paragraph_eid}__item_{Num}`
//! - 附則      → every node under the k-th `<SupplProvision>` (1-based among
//!   siblings) gets the prefix `suppl_{k}__`, e.g. `suppl_1__art_3__para_1`.
//!
//! `node_type` is one of `article` | `paragraph` | `item`. `sentence_text` is the
//! node's directly-owned `<Sentence>` text:
//! - paragraph: the sentences inside its own `<ParagraphSentence>`,
//! - item: the sentences inside its own `<ItemSentence>`,
//! - article with no paragraphs: its body text excluding `<ArticleCaption>`.
//!
//! `Num` is an XML attribute on `<Article>` / `<Paragraph>` / `<Item>` in the law
//! standard; we read it off the start tag. 枝番 such as 第9条の2 appear as
//! `Num="9_2"` and flow into the eId verbatim.

use std::collections::BTreeMap;

use quick_xml::Reader;
use quick_xml::events::{BytesStart, Event};

/// A normative node extracted from the law XML, keyed externally by its eId.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Node {
    pub eid: String,
    pub node_type: NodeType,
    /// The `Num` token as it appears in the source (or a synthesized ordinal).
    pub num: String,
    /// The joined normative `<Sentence>` text directly owned by this node.
    pub sentence_text: String,
    /// Caption text (`<ArticleCaption>`), tracked so caption-only changes can be
    /// classified as administrative. Empty for non-article nodes.
    pub caption: String,
    /// Parent eId, used to detect MOVED. Empty for top-level articles.
    pub parent_eid: String,
    /// Document order, used for stable output ordering.
    pub ordinal: usize,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NodeType {
    Article,
    Paragraph,
    Item,
}

impl NodeType {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Article => "article",
            Self::Paragraph => "paragraph",
            Self::Item => "item",
        }
    }
}

/// The result of parsing one snapshot.
#[derive(Debug, Default)]
pub struct ParsedLaw {
    /// eId → node, ordered by eId for deterministic iteration.
    pub nodes: BTreeMap<String, Node>,
    /// True when parsing hit a structural problem and the node map is degraded.
    /// Drives `class_confidence = low`.
    pub degraded: bool,
}

#[derive(Debug)]
pub enum ParseError {
    Xml(quick_xml::Error),
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Xml(e) => write!(f, "xml parse error: {e}"),
        }
    }
}

impl std::error::Error for ParseError {}

impl From<quick_xml::Error> for ParseError {
    fn from(e: quick_xml::Error) -> Self {
        Self::Xml(e)
    }
}

/// Parse a 法令標準XML byte slice into an eId-keyed node map.
///
/// An empty slice yields an empty `ParsedLaw` (used for the first-revision case
/// where `prev_snapshot` is `[]`).
pub fn parse(xml: &[u8]) -> Result<ParsedLaw, ParseError> {
    if xml.is_empty() {
        return Ok(ParsedLaw::default());
    }

    let mut reader = Reader::from_reader(xml);
    let cfg = reader.config_mut();
    cfg.trim_text(true);

    let mut builder = Builder::default();
    let mut buf = Vec::new();

    loop {
        match reader.read_event_into(&mut buf) {
            Ok(Event::Eof) => break,
            Ok(Event::Start(e)) => {
                let name = local_name(e.name().as_ref());
                let num = num_attr(&e);
                builder.on_start(&name, num);
            }
            Ok(Event::Empty(e)) => {
                // Self-closing tags carry no text; treat as start+end so Num-bearing
                // empty leaves still register (rare, but keeps the model consistent).
                let name = local_name(e.name().as_ref());
                let num = num_attr(&e);
                builder.on_start(&name, num);
                builder.on_end(&name);
            }
            Ok(Event::End(e)) => {
                let name = local_name(e.name().as_ref());
                builder.on_end(&name);
            }
            Ok(Event::Text(e)) => {
                // quick-xml 0.40 emits entity references as separate GeneralRef
                // events, so Text content here is already plain decoded text.
                let text = match e.decode() {
                    Ok(c) => c.into_owned(),
                    Err(_) => String::from_utf8_lossy(e.as_ref()).into_owned(),
                };
                builder.on_text(&text);
            }
            Ok(Event::GeneralRef(r)) => {
                // Resolve the five predefined XML entities; anything else is passed
                // through verbatim so we never drop normative characters.
                let raw = String::from_utf8_lossy(r.as_ref());
                let resolved = match raw.as_ref() {
                    "amp" => "&",
                    "lt" => "<",
                    "gt" => ">",
                    "quot" => "\"",
                    "apos" => "'",
                    other => {
                        // Numeric character references (&#...;) carry a leading '#'.
                        if let Some(decoded) = decode_char_ref(other) {
                            builder.on_text(&decoded);
                            buf.clear();
                            continue;
                        }
                        other
                    }
                };
                builder.on_text(resolved);
            }
            Ok(_) => {}
            Err(e) => return Err(ParseError::Xml(e)),
        }
        buf.clear();
    }

    Ok(builder.finish())
}

/// Strip an XML namespace prefix, returning the local element name.
fn local_name(raw: &[u8]) -> String {
    let name = String::from_utf8_lossy(raw);
    match name.rsplit_once(':') {
        Some((_, local)) => local.to_string(),
        None => name.into_owned(),
    }
}

/// Decode a numeric character reference body (`#123` or `#x1F`) to its character.
fn decode_char_ref(body: &str) -> Option<String> {
    let rest = body.strip_prefix('#')?;
    let code = if let Some(hex) = rest.strip_prefix(['x', 'X']) {
        u32::from_str_radix(hex, 16).ok()?
    } else {
        rest.parse::<u32>().ok()?
    };
    char::from_u32(code).map(|c| c.to_string())
}

/// Read the `Num` attribute off a start tag, if present.
fn num_attr(e: &BytesStart<'_>) -> Option<String> {
    for attr in e.attributes().flatten() {
        if attr.key.as_ref() == b"Num" {
            return Some(String::from_utf8_lossy(&attr.value).into_owned());
        }
    }
    None
}

/// Normalize a `Num` token for eId use. The law standard already encodes 枝番 as
/// `9_2`; we keep it verbatim and only trim surrounding whitespace.
fn normalize_num(num: &str) -> String {
    num.trim().to_string()
}

/// Where the current text run should be appended.
#[derive(Clone, Copy, PartialEq, Eq, Default)]
enum TextSink {
    #[default]
    None,
    /// Inside `<ArticleTitle>` / other non-normative label text — discard.
    Discard,
    ArticleCaption,
    ParagraphSentence,
    ItemSentence,
}

/// Streaming state machine that turns SAX-style events into the node map.
///
/// We track the current 附則 prefix, the open Article/Paragraph/Item context, and
/// route `<Sentence>` text into the correct owner.
#[derive(Default)]
struct Builder {
    law: ParsedLaw,
    ordinal: usize,

    /// 1-based index of the current `<SupplProvision>`, 0 when in MainProvision.
    suppl_index: usize,
    in_suppl: bool,

    cur_article: Option<OpenArticle>,
    cur_paragraph: Option<OpenParagraph>,
    cur_item: Option<OpenItem>,

    /// Whether the current article has emitted at least one paragraph; if not,
    /// the article owns body text directly.
    article_has_paragraph: bool,

    sink: TextSink,
}

struct OpenArticle {
    eid: String,
    num: String,
    caption: String,
    /// Body text for paragraph-less articles (sentences directly under Article).
    body: String,
    ordinal: usize,
    para_ordinal: usize,
}

struct OpenParagraph {
    eid: String,
    num: String,
    text: String,
    ordinal: usize,
    item_ordinal: usize,
}

struct OpenItem {
    eid: String,
    num: String,
    text: String,
    ordinal: usize,
}

impl Builder {
    fn next_ordinal(&mut self) -> usize {
        self.ordinal += 1;
        self.ordinal
    }

    fn prefix(&self) -> String {
        if self.in_suppl {
            format!("suppl_{}__", self.suppl_index)
        } else {
            String::new()
        }
    }

    fn on_start(&mut self, name: &str, num: Option<String>) {
        match name {
            "SupplProvision" => {
                self.in_suppl = true;
                self.suppl_index += 1;
            }
            "Article" => {
                let num = num.unwrap_or_default();
                let eid = format!("{}art_{}", self.prefix(), normalize_num(&num));
                let ordinal = self.next_ordinal();
                self.cur_article = Some(OpenArticle {
                    eid,
                    num,
                    caption: String::new(),
                    body: String::new(),
                    ordinal,
                    para_ordinal: 0,
                });
                self.article_has_paragraph = false;
            }
            "ArticleCaption" => self.sink = TextSink::ArticleCaption,
            // Label/title text is non-normative; route it to the discard sink so it
            // never leaks into a paragraph-less article's body.
            "ArticleTitle" | "ParagraphNum" | "ItemTitle" => self.sink = TextSink::Discard,
            "Paragraph" => {
                if let Some(article) = self.cur_article.as_mut() {
                    self.article_has_paragraph = true;
                    article.para_ordinal += 1;
                    let num = num
                        .filter(|n| !n.trim().is_empty())
                        .unwrap_or_else(|| article.para_ordinal.to_string());
                    let eid = format!("{}__para_{}", article.eid, normalize_num(&num));
                    let ordinal = self.next_ordinal();
                    self.cur_paragraph = Some(OpenParagraph {
                        eid,
                        num,
                        text: String::new(),
                        ordinal,
                        item_ordinal: 0,
                    });
                }
            }
            "ParagraphSentence" => self.sink = TextSink::ParagraphSentence,
            "Item" => {
                if let Some(para) = self.cur_paragraph.as_mut() {
                    para.item_ordinal += 1;
                    let num = num
                        .filter(|n| !n.trim().is_empty())
                        .unwrap_or_else(|| para.item_ordinal.to_string());
                    let eid = format!("{}__item_{}", para.eid, normalize_num(&num));
                    let ordinal = self.next_ordinal();
                    self.cur_item = Some(OpenItem {
                        eid,
                        num,
                        text: String::new(),
                        ordinal,
                    });
                }
            }
            "ItemSentence" => self.sink = TextSink::ItemSentence,
            _ => {}
        }
    }

    fn on_end(&mut self, name: &str) {
        match name {
            "SupplProvision" => self.in_suppl = false,
            "ArticleCaption" | "ParagraphSentence" | "ItemSentence" | "ArticleTitle"
            | "ParagraphNum" | "ItemTitle" => self.sink = TextSink::None,
            "Item" => {
                if let Some(item) = self.cur_item.take() {
                    let parent_eid = self
                        .cur_paragraph
                        .as_ref()
                        .map(|p| p.eid.clone())
                        .unwrap_or_default();
                    self.push(Node {
                        eid: item.eid,
                        node_type: NodeType::Item,
                        num: item.num,
                        sentence_text: item.text.trim().to_string(),
                        caption: String::new(),
                        parent_eid,
                        ordinal: item.ordinal,
                    });
                }
            }
            "Paragraph" => {
                if let Some(para) = self.cur_paragraph.take() {
                    let parent_eid = self
                        .cur_article
                        .as_ref()
                        .map(|a| a.eid.clone())
                        .unwrap_or_default();
                    self.push(Node {
                        eid: para.eid,
                        node_type: NodeType::Paragraph,
                        num: para.num,
                        sentence_text: para.text.trim().to_string(),
                        caption: String::new(),
                        parent_eid,
                        ordinal: para.ordinal,
                    });
                }
            }
            "Article" => {
                if let Some(article) = self.cur_article.take() {
                    // An article with paragraphs owns no direct sentence text; one
                    // without paragraphs owns its body text directly.
                    let sentence_text = if self.article_has_paragraph {
                        String::new()
                    } else {
                        article.body.trim().to_string()
                    };
                    self.push(Node {
                        eid: article.eid,
                        node_type: NodeType::Article,
                        num: article.num,
                        sentence_text,
                        caption: article.caption.trim().to_string(),
                        parent_eid: String::new(),
                        ordinal: article.ordinal,
                    });
                }
            }
            _ => {}
        }
    }

    fn on_text(&mut self, text: &str) {
        match self.sink {
            TextSink::Discard => {}
            TextSink::None => {
                // Text outside a recognized sink. If we are inside a paragraph-less
                // article (no open paragraph/item), this is the article's own body
                // text (e.g. a Sentence placed directly under the Article).
                if self.cur_paragraph.is_none()
                    && self.cur_item.is_none()
                    && !self.article_has_paragraph
                    && let Some(a) = self.cur_article.as_mut()
                {
                    a.body.push_str(text);
                }
            }
            TextSink::ArticleCaption => {
                if let Some(a) = self.cur_article.as_mut() {
                    a.caption.push_str(text);
                }
            }
            TextSink::ParagraphSentence => {
                if let Some(p) = self.cur_paragraph.as_mut() {
                    p.text.push_str(text);
                }
            }
            TextSink::ItemSentence => {
                if let Some(i) = self.cur_item.as_mut() {
                    i.text.push_str(text);
                }
            }
        }
    }

    fn push(&mut self, node: Node) {
        if self.law.nodes.insert(node.eid.clone(), node).is_some() {
            // Duplicate eId means a malformed law or a derivation bug — flag
            // degraded so the classifier drops to low confidence.
            self.law.degraded = true;
        }
    }

    fn finish(mut self) -> ParsedLaw {
        // Nothing should remain open in well-formed XML; if it is, mark degraded.
        if self.cur_article.is_some() || self.cur_paragraph.is_some() || self.cur_item.is_some() {
            self.law.degraded = true;
        }
        self.law
    }
}
