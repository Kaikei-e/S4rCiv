-- M4 interpretation read model for the giin-roster (両院公式議員名簿) adapter (ADR-000008).
-- Tier 2 disposable projection (ADR-000002): NO mutation guard — the projector
-- rebuilds it in place (TRUNCATE -> reset projector_offset -> replay). Each row
-- carries provenance (observation_seq) into the immutable ground truth and an
-- event-derived observed_at (never a projector wall-clock — invariant 3).
--
-- This binds a legislator (Popolo person_id) to the electoral district they were
-- elected from, so the district vote map can colour a 記名投票 by each district's
-- current member. Legislators are accountable public officials, so this geographic
-- binding is NOT private-person profiling (DISCIPLINE §4; ADR-000006 endorses
-- "自分の代表がどう投票したか" as the accountability core).
--
-- person_id is the SAME deterministic id as interpretation.person (derived by the
-- same Popolo identity function in the gateway), but is joined BY VALUE, not by a
-- foreign key: read models reference observation only, never each other, so each
-- can be truncated independently during a reproject (kokkai read-model invariant).
--
-- 現会期スコープ (ADR-000008): the roster holds CURRENT members only, so the map is
-- a present-tense lens. Historical votes are kept by the immutable log / Timeline,
-- not here.

CREATE TABLE interpretation.legislator_district (
  person_id           text PRIMARY KEY,          -- == interpretation.person.person_id (join by value, NOT a FK)
  stream_id           text NOT NULL,             -- the giin-roster page Stream this row came from (replaced as a unit on ResourceChanged)
  name                text NOT NULL,
  house               text NOT NULL,             -- 衆議院 | 参議院
  district_code       text,                      -- 国土数値情報-aligned district code; NULL for 比例
  district_name       text,                      -- 人間可読 (東京1区 / 東京都); NULL for 比例
  is_pr               boolean NOT NULL DEFAULT false, -- 比例選出 (no district; shown in the companion panel, never erased — §5)
  pr_block            text,                      -- 比例ブロック (衆: 11 blocks, 参: 全国); NULL when not PR
  parliamentary_group text,                      -- 会派
  observation_seq     bigint NOT NULL REFERENCES observation.event (seq),
  observed_at         timestamptz NOT NULL,      -- event-time fetch timestamp
  CONSTRAINT legislator_district_house_known
    CHECK (house IN ('衆議院', '参議院')),
  -- The geometry binding invariant: a district member has a district_code, a 比例
  -- member has none. Keeps the choropleth join total (no half-bound rows).
  CONSTRAINT legislator_district_pr_consistent
    CHECK ((is_pr AND district_code IS NULL) OR (NOT is_pr AND district_code IS NOT NULL))
);

-- Reverse lookup (district -> current member) for coverage/panel queries.
CREATE INDEX legislator_district_code_idx ON interpretation.legislator_district (district_code);
-- Per-page replacement on re-observation (ApplyRoster deletes a page's rows by stream_id).
CREATE INDEX legislator_district_stream_idx ON interpretation.legislator_district (stream_id);
