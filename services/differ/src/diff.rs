//! Structural diff and administrative/substantive classification of two law
//! snapshots, working purely on the eId-keyed node maps from `xmlmodel`.
//!
//! Diff rules (per ADR-000005 contract):
//! - eId in curr only                          → ADDED   (curr_text set)
//! - eId in prev only                          → DELETED (prev_text set)
//! - eId in both, sentence_text differs        → MODIFIED (both texts set)
//! - eId in both, same sentence_text but a
//!   different parent_eid                       → MOVED
//!
//! Classification (v0 — 本文 vs 非本文, conservative, fail toward review):
//! - `substantive` if any ADDED/DELETED/MODIFIED node carries a real change in the
//!   normative sentence_text of an article/paragraph/item.
//! - `administrative` only when the sole differences are non-normative: caption-only
//!   edits, pure renumbering with identical text, whitespace-only.
//! - When uncertain → `substantive`.

use crate::xmlmodel::{Node, NodeType, ParsedLaw};

pub const DIFFER_VERSION: &str = "egov-akn-diff/0.1.0";

/// Mirror of the proto `ChangeOp` as a plain Rust enum so the diff logic stays
/// independent of the generated types; mapped to proto at the RPC boundary.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ChangeOp {
    Added,
    Deleted,
    Modified,
    Moved,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NodeChange {
    pub eid: String,
    pub op: ChangeOp,
    pub node_type: NodeType,
    pub num: String,
    pub prev_text: String,
    pub curr_text: String,
    /// Stable sort key: curr ordinal when present, else prev ordinal.
    ordinal: usize,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Classification {
    Administrative,
    Substantive,
}

impl Classification {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Administrative => "administrative",
            Self::Substantive => "substantive",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Confidence {
    High,
    Medium,
    Low,
}

impl Confidence {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::High => "high",
            Self::Medium => "medium",
            Self::Low => "low",
        }
    }
}

#[derive(Debug)]
pub struct DiffResult {
    pub changes: Vec<NodeChange>,
    pub classification: Classification,
    pub confidence: Confidence,
}

/// Compute the node-level diff and classification between two parsed snapshots.
pub fn compute(prev: &ParsedLaw, curr: &ParsedLaw) -> DiffResult {
    let mut changes: Vec<NodeChange> = Vec::new();

    // ADDED / MODIFIED / MOVED: walk curr against prev.
    for (eid, curr_node) in &curr.nodes {
        match prev.nodes.get(eid) {
            None => changes.push(added(curr_node)),
            Some(prev_node) => {
                if prev_node.sentence_text != curr_node.sentence_text {
                    changes.push(modified(prev_node, curr_node));
                } else if prev_node.parent_eid != curr_node.parent_eid {
                    changes.push(moved(prev_node, curr_node));
                }
                // Caption-only differences are not normative; they do not produce a
                // NodeChange but do influence confidence (handled below).
            }
        }
    }

    // DELETED: eIds present only in prev.
    for (eid, prev_node) in &prev.nodes {
        if !curr.nodes.contains_key(eid) {
            changes.push(deleted(prev_node));
        }
    }

    changes.sort_by(|a, b| a.ordinal.cmp(&b.ordinal).then_with(|| a.eid.cmp(&b.eid)));

    let caption_changed = caption_only_change(prev, curr);
    let classification = classify(&changes);
    let confidence = confidence(&changes, classification, caption_changed, prev, curr);

    DiffResult {
        changes,
        classification,
        confidence,
    }
}

fn added(n: &Node) -> NodeChange {
    NodeChange {
        eid: n.eid.clone(),
        op: ChangeOp::Added,
        node_type: n.node_type,
        num: n.num.clone(),
        prev_text: String::new(),
        curr_text: n.sentence_text.clone(),
        ordinal: n.ordinal,
    }
}

fn deleted(n: &Node) -> NodeChange {
    NodeChange {
        eid: n.eid.clone(),
        op: ChangeOp::Deleted,
        node_type: n.node_type,
        num: n.num.clone(),
        prev_text: n.sentence_text.clone(),
        curr_text: String::new(),
        ordinal: n.ordinal,
    }
}

fn modified(prev: &Node, curr: &Node) -> NodeChange {
    NodeChange {
        eid: curr.eid.clone(),
        op: ChangeOp::Modified,
        node_type: curr.node_type,
        num: curr.num.clone(),
        prev_text: prev.sentence_text.clone(),
        curr_text: curr.sentence_text.clone(),
        ordinal: curr.ordinal,
    }
}

fn moved(prev: &Node, curr: &Node) -> NodeChange {
    NodeChange {
        eid: curr.eid.clone(),
        op: ChangeOp::Moved,
        node_type: curr.node_type,
        num: curr.num.clone(),
        prev_text: prev.sentence_text.clone(),
        curr_text: curr.sentence_text.clone(),
        ordinal: curr.ordinal,
    }
}

/// A change is normative when it adds, removes, or alters the sentence_text of an
/// article/paragraph/item. ADDED/DELETED of a text-bearing node count; an ADDED node
/// with empty text (e.g. a paragraph-less article that only had a caption) does not.
fn is_normative(c: &NodeChange) -> bool {
    let bearing = matches!(
        c.node_type,
        NodeType::Article | NodeType::Paragraph | NodeType::Item
    );
    if !bearing {
        return false;
    }
    match c.op {
        ChangeOp::Added => !c.curr_text.is_empty(),
        ChangeOp::Deleted => !c.prev_text.is_empty(),
        ChangeOp::Modified => c.prev_text != c.curr_text,
        // A pure move with identical text is renumbering, not a normative change.
        ChangeOp::Moved => false,
    }
}

fn classify(changes: &[NodeChange]) -> Classification {
    if changes.iter().any(is_normative) {
        Classification::Substantive
    } else {
        Classification::Administrative
    }
}

/// Detect whether any node common to both snapshots changed only its caption (no
/// normative text change). Used to keep confidence high for clearly-cosmetic edits.
fn caption_only_change(prev: &ParsedLaw, curr: &ParsedLaw) -> bool {
    for (eid, curr_node) in &curr.nodes {
        if let Some(prev_node) = prev.nodes.get(eid)
            && prev_node.caption != curr_node.caption
            && prev_node.sentence_text == curr_node.sentence_text
        {
            return true;
        }
    }
    false
}

fn confidence(
    changes: &[NodeChange],
    classification: Classification,
    caption_changed: bool,
    prev: &ParsedLaw,
    curr: &ParsedLaw,
) -> Confidence {
    // Degraded parsing dominates — we cannot trust the node map.
    if prev.degraded || curr.degraded {
        return Confidence::Low;
    }
    match classification {
        // At least one clear text-bearing change → high.
        Classification::Substantive if changes.iter().any(is_normative) => Confidence::High,
        // No diff at all, or only caption-level cosmetic changes → high.
        Classification::Administrative if changes.is_empty() || caption_changed => Confidence::High,
        // Otherwise (e.g. structure-only moves we could not fully reason about) →
        // medium.
        _ => Confidence::Medium,
    }
}
