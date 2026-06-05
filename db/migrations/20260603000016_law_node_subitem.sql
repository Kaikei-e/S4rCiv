-- Extend interpretation.law_node to carry 号の細分 (Subitem イ・ロ・(1)(2)…) as their
-- own normative nodes. The 法令標準XML nests these under <Subitem1>/<Subitem2>/… inside
-- an <Item>; before this migration the projector and the differ dropped them entirely,
-- so a change to a sub-item produced no detected change (silence about a real change)
-- and 用語定義号 that carry their term/意義 in <Column> rendered with no text.
--
-- A new node_type 'subitem' is admitted. Depth is encoded in the eId, not the type:
-- e.g. art_14__para_1__item_2__subitem1_2__subitem2_1 (the same identity the Go
-- projector and the Rust differ derive — ADR-000005 / ADR-000013). law_node is a Tier-2
-- disposable read model, so existing rows are unaffected and the new sub-item rows
-- appear on the next reproject; only the CHECK domain widens here.

ALTER TABLE interpretation.law_node
  DROP CONSTRAINT law_node_type_known,
  ADD CONSTRAINT law_node_type_known
    CHECK (node_type IN ('article', 'paragraph', 'item', 'suppl_provision', 'subitem'));
