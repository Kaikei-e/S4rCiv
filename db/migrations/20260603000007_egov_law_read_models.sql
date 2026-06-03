-- Interpretation read models for the e-Gov 法令 (egov-law) adapter.
-- Tier 2 disposable projections (ADR-000002): NO mutation guard — the law entity
-- projector rebuilds these in place (TRUNCATE -> reset projector_offset -> replay).
-- Each row carries provenance (observation_seq) into the immutable ground truth and
-- an event-derived observed_at (never a projector wall-clock — invariant 3).
--
-- A law is a living document: one stream observes its 現行 (in-force consolidated)
-- text, so the normalized tree here reflects only the CURRENT snapshot and is
-- rebuilt on every change (current-tree-only; ADR-000002 rejects versioned rows).
-- How the text *changed* over time lives in interpretation.change, computed by the
-- differ service over consecutive snapshots (ADR-000005), not here.

-- ── Akoma-Ntoso-aligned legislative work (law-level metadata) ────────────────────
-- One row per law, keyed by the stable e-Gov 法令ID (e.g. '415AC0000000057').
CREATE TABLE interpretation.legislative_work (
  law_id                text PRIMARY KEY,        -- e-Gov 法令ID, stable across revisions
  stream_id             text NOT NULL,           -- 'egov-law:<law_id>'
  law_num               text,                    -- 法令番号 (e.g. 平成十五年法律第五十七号)
  law_type              text,                    -- e-Gov enum: Act / CabinetOrder / ...
  law_title             text,
  law_title_kana        text,
  category              text,                    -- e-Gov 分類 (e.g. 刑事)
  promulgation_date     date,                    -- 公布日
  current_revision_id   text,                    -- observed law_revision_id ({law_id}_{施行日}_{改正法令ID})
  amendment_promulgate_date  date,               -- 当該版の改正公布日
  amendment_enforcement_date date,               -- 当該版の施行日
  current_revision_status text,                  -- e-Gov: CurrentEnforced / ... (in-force status)
  repeal_status         text,                    -- e-Gov: None / Repeal / ...
  repeal_date           date,
  permalink             text,                    -- e-Gov reference URL (attribution; DISCIPLINE §9)
  was_ocr               boolean NOT NULL DEFAULT false,  -- always false for XML, kept for attribution parity
  observation_seq       bigint NOT NULL REFERENCES observation.event (seq),
  observed_at           timestamptz NOT NULL
);
CREATE INDEX legislative_work_type_idx ON interpretation.legislative_work (law_type);
CREATE INDEX legislative_work_promulgation_idx ON interpretation.legislative_work (promulgation_date);

-- ── Normalized AKN structure nodes (条/項/号), current tree only ──────────────────
-- eid is the stable per-node identity within a law (AKN element id, e.g.
-- 'art_9__para_1__item_2'). It is derived identically by the Go projector (these
-- rows) and the Rust differ (interpretation.change node refs), so a reported change
-- joins to its node row (ADR-000005 shared-identity contract). Container levels
-- (編/章/節/款/目) are carried as metadata columns rather than their own rows.
CREATE TABLE interpretation.law_node (
  id              bigserial PRIMARY KEY,
  law_id          text NOT NULL REFERENCES interpretation.legislative_work (law_id),
  eid             text NOT NULL,                 -- AKN element id, unique within the law
  parent_eid      text,                          -- tree link (NULL at top of MainProvision)
  node_type       text NOT NULL,                 -- 'article' | 'paragraph' | 'item' | 'suppl_provision'
  num             text,                          -- 条/項/号 number token ('9', '9_2' for 第9条の2)
  caption         text,                          -- 見出し (ArticleCaption), when present
  chapter_num     text,                          -- container metadata
  section_num     text,                          -- container metadata
  is_suppl        boolean NOT NULL DEFAULT false, -- 附則 (SupplProvision) vs 本則 (MainProvision)
  sentence_text   text,                          -- normative text of the node (art. 13 free; full-text OK)
  ordinal         integer NOT NULL,              -- stable document order within the law
  observation_seq bigint NOT NULL REFERENCES observation.event (seq),
  observed_at     timestamptz NOT NULL,
  CONSTRAINT law_node_eid_unique UNIQUE (law_id, eid),
  CONSTRAINT law_node_type_known
    CHECK (node_type IN ('article', 'paragraph', 'item', 'suppl_provision'))
);
CREATE INDEX law_node_law_order_idx ON interpretation.law_node (law_id, ordinal);
CREATE INDEX law_node_eid_idx ON interpretation.law_node (eid);
